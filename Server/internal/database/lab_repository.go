package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/G6kco/CyberSpace/internal/labs"
)

// LabRepository stores lab instances, their addresses, and the orchestration
// jobs the labs.Engine runs.
//
// Every timestamp is written explicitly as UTC_TIMESTAMP(6): the schema's
// CHECK constraints compare them with each other and with created_at, so
// mixing in the session clock's CURRENT_TIMESTAMP could fail those checks.
type LabRepository struct {
	db *sql.DB
}

// provisionRetries bounds how often RequestProvision retries a transaction
// InnoDB rolled back to break a deadlock with a concurrent stop.
const provisionRetries = 3

// failureMessageLimit is the length of lab_instances.failure_message.
const failureMessageLimit = 1000

func NewLabRepository(db *sql.DB) *LabRepository {
	return &LabRepository{db: db}
}

func (r *LabRepository) RequestProvision(
	ctx context.Context,
	attemptID, templateVersionID uint64,
) (uint64, error) {
	for try := 1; ; try++ {
		instanceID, err := r.requestProvision(ctx, attemptID, templateVersionID)
		if err == nil || try == provisionRetries || !isDeadlock(err) {
			return instanceID, err
		}
	}
}

func (r *LabRepository) requestProvision(
	ctx context.Context,
	attemptID, templateVersionID uint64,
) (uint64, error) {
	// READ COMMITTED, so each read below sees allocations committed by the
	// starts that held the host lock before this one.
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Starts are serialized on the host rows. Picking an address with
	// FOR UPDATE SKIP LOCKED instead also locks the allocated rows it scans
	// and rejects, so concurrent starts skipped free addresses and failed;
	// it also let them overshoot max_instances together. Each start holds
	// the lock for a few milliseconds, so a full room queues briefly.
	hosts, err := tx.QueryContext(ctx,
		`SELECT id FROM docker_hosts WHERE status = 'online' FOR UPDATE`,
	)
	if err != nil {
		return 0, err
	}
	if err := hosts.Close(); err != nil {
		return 0, err
	}

	// One active lab per attempt is left to uq_lab_instances_one_active
	// rather than checked with a locking read: locking a row that does not
	// exist takes a gap lock, and concurrent starts then deadlock on insert.
	var addressID, hostID uint64
	err = tx.QueryRowContext(ctx,
		`SELECT na.id, np.docker_host_id
        FROM network_addresses na
        JOIN network_pools np ON np.id = na.network_pool_id
        JOIN docker_hosts dh ON dh.id = np.docker_host_id
        WHERE na.is_reserved = FALSE
          AND np.status = 'active'
          AND dh.status = 'online'
          AND NOT EXISTS (
              SELECT 1 FROM ip_allocations ia
              WHERE ia.active_network_address_id = na.id
          )
          AND (
              SELECT COUNT(*) FROM lab_instances li
              WHERE li.docker_host_id = dh.id
                AND li.active_attempt_id IS NOT NULL
          ) < dh.max_instances
        ORDER BY na.id
        LIMIT 1`,
	).Scan(&addressID, &hostID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, labs.ErrNoFreeAddress
	}
	if err != nil {
		return 0, err
	}

	var generation uint64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(generation_no), 0) + 1
        FROM lab_instances
        WHERE attempt_id = ?`,
		attemptID,
	).Scan(&generation); err != nil {
		return 0, err
	}

	result, err := tx.ExecContext(ctx,
		`INSERT INTO lab_instances
        (attempt_id, lab_template_version_id, docker_host_id, generation_no,
         status, created_at)
        VALUES (?, ?, ?, ?, 'queued', UTC_TIMESTAMP(6))`,
		attemptID, templateVersionID, hostID, generation,
	)
	if isDuplicate(err, "uq_lab_instances_one_active") {
		return 0, labs.ErrLabActive
	}
	instanceID, err := insertedID(result, err)
	if err != nil {
		return 0, err
	}

	serviceID, err := insertedID(tx.ExecContext(ctx,
		`INSERT INTO lab_instance_services
        (lab_instance_id, service_name, state, internal_hostname, created_at)
        VALUES (?, ?, 'pending', ?, UTC_TIMESTAMP(6))`,
		instanceID, labs.TargetService, labs.ContainerName(instanceID),
	))
	if err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO ip_allocations
        (network_address_id, lab_instance_service_id, allocated_at)
        VALUES (?, ?, UTC_TIMESTAMP(6))`,
		addressID, serviceID,
	); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO orchestration_jobs
        (lab_instance_id, job_type, status, idempotency_key, available_at, created_at)
        VALUES (?, ?, 'pending', ?, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))`,
		instanceID, labs.JobProvision, jobKey(labs.JobProvision, instanceID),
	); err != nil {
		return 0, err
	}

	return instanceID, tx.Commit()
}

func (r *LabRepository) RequestStop(ctx context.Context, attemptID uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var instanceID uint64
	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT id, status FROM lab_instances WHERE active_attempt_id = ? FOR UPDATE`,
		attemptID,
	).Scan(&instanceID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	// A provision job no worker has claimed yet is simply withdrawn.
	withdrawn, err := tx.ExecContext(ctx,
		`UPDATE orchestration_jobs
        SET status = 'cancelled', completed_at = UTC_TIMESTAMP(6)
        WHERE lab_instance_id = ? AND job_type = ? AND status = 'pending'`,
		instanceID, labs.JobProvision,
	)
	if err != nil {
		return err
	}

	// A queued lab whose job was withdrawn never had a container, so it is
	// finished here; anything further along needs a stop job to remove one.
	if n, _ := withdrawn.RowsAffected(); status == "queued" && n > 0 {
		if err := finishInstance(ctx, tx, instanceID, "stopped", "removed", nil); err != nil {
			return err
		}
		return tx.Commit()
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE lab_instances SET status = 'stopping' WHERE id = ?`,
		instanceID,
	); err != nil {
		return err
	}

	// A stop job that already exists is left alone, unless it failed: then
	// it is reset so a stop that failed while Docker was down runs again.
	// MySQL evaluates the assignments left to right, so status is set last
	// for the others to see its old value.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO orchestration_jobs
        (lab_instance_id, job_type, status, idempotency_key, available_at, created_at)
        VALUES (?, ?, 'pending', ?, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))
        ON DUPLICATE KEY UPDATE
            attempt_count = IF(status = 'failed', 0, attempt_count),
            available_at = IF(status = 'failed', UTC_TIMESTAMP(6), available_at),
            completed_at = IF(status = 'failed', NULL, completed_at),
            locked_at = IF(status = 'failed', NULL, locked_at),
            locked_by = IF(status = 'failed', NULL, locked_by),
            status = IF(status = 'failed', 'pending', status)`,
		instanceID, labs.JobStop, jobKey(labs.JobStop, instanceID),
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *LabRepository) ClaimJob(
	ctx context.Context,
	workerID string,
	lease time.Duration,
) (labs.Job, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return labs.Job{}, false, err
	}
	defer tx.Rollback()

	// A running job whose lease has passed belongs to a worker that died,
	// so it is taken over like a pending one.
	var job labs.Job
	err = tx.QueryRowContext(ctx,
		`SELECT id, lab_instance_id, job_type, attempt_count, max_attempts
        FROM orchestration_jobs
        WHERE attempt_count < max_attempts
          AND (
              (status = 'pending' AND available_at <= UTC_TIMESTAMP(6))
              OR
              (status = 'running' AND locked_at < UTC_TIMESTAMP(6) - INTERVAL ? MICROSECOND)
          )
        ORDER BY available_at, id
        LIMIT 1
        FOR UPDATE SKIP LOCKED`,
		lease.Microseconds(),
	).Scan(&job.ID, &job.InstanceID, &job.Type, &job.AttemptCount, &job.MaxAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return labs.Job{}, false, nil
	}
	if err != nil {
		return labs.Job{}, false, err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE orchestration_jobs
        SET status = 'running',
            attempt_count = attempt_count + 1,
            locked_at = UTC_TIMESTAMP(6),
            locked_by = ?
        WHERE id = ?`,
		workerID, job.ID,
	); err != nil {
		return labs.Job{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return labs.Job{}, false, err
	}

	job.AttemptCount++
	return job, true, nil
}

func (r *LabRepository) RetryJob(
	ctx context.Context,
	jobID uint64,
	delay time.Duration,
	cause error,
) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE orchestration_jobs
        SET status = 'pending',
            available_at = UTC_TIMESTAMP(6) + INTERVAL ? MICROSECOND,
            locked_at = NULL,
            locked_by = NULL,
            last_error = ?
        WHERE id = ? AND status = 'running'`,
		delay.Microseconds(), cause.Error(), jobID,
	)
	return err
}

func (r *LabRepository) CompleteJob(
	ctx context.Context,
	jobID uint64,
	status string,
	cause error,
) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE orchestration_jobs
        SET status = ?,
            completed_at = UTC_TIMESTAMP(6),
            last_error = COALESCE(?, last_error)
        WHERE id = ? AND status = 'running'`,
		status, errorText(cause), jobID,
	)
	return err
}

func (r *LabRepository) FailProvision(ctx context.Context, job labs.Job, cause error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE orchestration_jobs
        SET status = 'failed', completed_at = UTC_TIMESTAMP(6), last_error = ?
        WHERE id = ? AND status = 'running'`,
		cause.Error(), job.ID,
	); err != nil {
		return err
	}

	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM lab_instances WHERE id = ? FOR UPDATE`,
		job.InstanceID,
	).Scan(&status)
	if err != nil {
		return err
	}
	// A stop requested meanwhile owns the instance; only the job fails.
	if status == "queued" || status == "provisioning" {
		if err := finishInstance(ctx, tx, job.InstanceID, "failed", "failed", cause); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *LabRepository) FailStaleJobs(ctx context.Context, lease time.Duration) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE orchestration_jobs
        SET status = 'failed',
            completed_at = UTC_TIMESTAMP(6),
            last_error = 'worker stopped responding on the final attempt'
        WHERE status = 'running'
          AND attempt_count >= max_attempts
          AND locked_at < UTC_TIMESTAMP(6) - INTERVAL ? MICROSECOND`,
		lease.Microseconds(),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *LabRepository) LoadTarget(ctx context.Context, instanceID uint64) (labs.Target, error) {
	target := labs.Target{InstanceID: instanceID}
	var spec, network sql.NullString
	var address []byte

	// Outer joins, so a stopped instance (whose address is released) still
	// reports its status and the engine can cancel the job.
	err := r.db.QueryRowContext(ctx,
		`SELECT li.status, ltv.yaml_spec, np.name, na.ip_address
        FROM lab_instances li
        JOIN lab_template_versions ltv ON ltv.id = li.lab_template_version_id
        LEFT JOIN lab_instance_services s
               ON s.lab_instance_id = li.id AND s.service_name = ?
        LEFT JOIN ip_allocations ia ON ia.active_service_id = s.id
        LEFT JOIN network_addresses na ON na.id = ia.network_address_id
        LEFT JOIN network_pools np ON np.id = na.network_pool_id
        WHERE li.id = ?`,
		labs.TargetService, instanceID,
	).Scan(&target.Status, &spec, &network, &address)
	if err != nil {
		return labs.Target{}, err
	}
	if target.Status != "queued" && target.Status != "provisioning" {
		return target, nil
	}

	if !network.Valid || address == nil {
		return labs.Target{}, errors.New("lab has no allocated address")
	}
	ip, ok := netip.AddrFromSlice(address)
	if !ok {
		return labs.Target{}, fmt.Errorf("stored lab address %x is not an IP address", address)
	}
	target.IP = ip.Unmap()
	target.Network = network.String

	target.Spec, err = labs.ParseSpec(spec.String)
	if err != nil {
		return labs.Target{}, err
	}
	return target, nil
}

func (r *LabRepository) MarkProvisioning(ctx context.Context, instanceID uint64) error {
	return r.transition(ctx, instanceID, []string{"queued", "provisioning"}, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE lab_instances
            SET status = 'provisioning',
                started_at = COALESCE(started_at, UTC_TIMESTAMP(6))
            WHERE id = ?`,
			instanceID,
		); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE lab_instance_services SET state = 'creating' WHERE lab_instance_id = ?`,
			instanceID,
		)
		return err
	})
}

func (r *LabRepository) MarkRunning(ctx context.Context, instanceID uint64, containerID string) error {
	return r.transition(ctx, instanceID, []string{"provisioning"}, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE lab_instances
            SET status = 'running', ready_at = UTC_TIMESTAMP(6)
            WHERE id = ?`,
			instanceID,
		); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE lab_instance_services
            SET state = 'running',
                container_id = ?,
                started_at = UTC_TIMESTAMP(6),
                stopped_at = NULL
            WHERE lab_instance_id = ? AND service_name = ?`,
			containerID, instanceID, labs.TargetService,
		)
		return err
	})
}

func (r *LabRepository) MarkStopped(ctx context.Context, instanceID uint64) error {
	err := r.transition(ctx, instanceID, []string{"stopping"}, func(tx *sql.Tx) error {
		return finishInstance(ctx, tx, instanceID, "stopped", "removed", nil)
	})
	// Already stopped or failed: the container is gone either way.
	if errors.Is(err, labs.ErrStateChanged) {
		return nil
	}
	return err
}

func (r *LabRepository) ExpiredAttempts(ctx context.Context) ([]uint64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT li.attempt_id
        FROM lab_instances li
        JOIN attempts a ON a.id = li.attempt_id
        WHERE li.active_attempt_id IS NOT NULL
          AND (
              a.status NOT IN ('starting', 'in_progress')
              OR a.deadline_at <= UTC_TIMESTAMP(6)
          )`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		attempts = append(attempts, id)
	}
	return attempts, rows.Err()
}

func (r *LabRepository) LiveInstances(ctx context.Context) (map[uint64]bool, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM lab_instances WHERE status IN ('provisioning', 'running', 'stopping')`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	live := make(map[uint64]bool)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		live[id] = true
	}
	return live, rows.Err()
}

// transition locks the instance and runs apply only when its status is one
// of from. An UPDATE's affected-row count cannot replace this check: MySQL
// reports zero for a row whose values did not change.
func (r *LabRepository) transition(
	ctx context.Context,
	instanceID uint64,
	from []string,
	apply func(*sql.Tx) error,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM lab_instances WHERE id = ? FOR UPDATE`,
		instanceID,
	).Scan(&status); err != nil {
		return err
	}
	allowed := false
	for _, s := range from {
		allowed = allowed || s == status
	}
	if !allowed {
		return fmt.Errorf("%w: instance %d is %s", labs.ErrStateChanged, instanceID, status)
	}

	if err := apply(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// finishInstance ends an instance, its services, and its address allocation.
// cause is recorded only for a failed instance. A service's stopped_at is
// set only if it started, as the schema requires.
func finishInstance(
	ctx context.Context,
	tx *sql.Tx,
	instanceID uint64,
	instanceStatus, serviceState string,
	cause error,
) error {
	var code, message any
	if instanceStatus == "failed" {
		code = "provision_failed"
		message = truncate(cause.Error(), failureMessageLimit)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE lab_instances
        SET status = ?, stopped_at = UTC_TIMESTAMP(6),
            failure_code = ?, failure_message = ?
        WHERE id = ?`,
		instanceStatus, code, message, instanceID,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE lab_instance_services
        SET state = ?,
            stopped_at = IF(started_at IS NULL, NULL, UTC_TIMESTAMP(6))
        WHERE lab_instance_id = ?`,
		serviceState, instanceID,
	); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx,
		`UPDATE ip_allocations ia
        JOIN lab_instance_services s ON s.id = ia.lab_instance_service_id
        SET ia.released_at = UTC_TIMESTAMP(6)
        WHERE s.lab_instance_id = ? AND ia.released_at IS NULL`,
		instanceID,
	)
	return err
}

func jobKey(jobType string, instanceID uint64) string {
	return jobType + ":" + strconv.FormatUint(instanceID, 10)
}

func insertedID(result sql.Result, err error) (uint64, error) {
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return uint64(id), err
}

// isDeadlock reports whether InnoDB rolled the transaction back to break a
// deadlock, after which it is safe to run again.
func isDeadlock(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1213
}

// isDuplicate reports whether err is MySQL's duplicate-key error for the
// named unique constraint.
func isDuplicate(err error, constraint string) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) &&
		mysqlErr.Number == 1062 &&
		strings.Contains(mysqlErr.Message, constraint)
}

func errorText(err error) any {
	if err == nil {
		return nil
	}
	return err.Error()
}

func truncate(s string, limit int) string {
	if r := []rune(s); len(r) > limit {
		return string(r[:limit])
	}
	return s
}
