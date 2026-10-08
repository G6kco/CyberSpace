package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/G6kco/CyberSpace/internal/labs"
)

// labFixture is the minimum chain of rows a lab instance needs: an approved
// grant and an attempt for a published assessment version, a published lab
// template version, and an online Docker host with a macvlan address pool.
type labFixture struct {
	db        *sql.DB
	repo      *LabRepository
	adminID   uint64
	versionID uint64
	labID     uint64
	poolID    uint64
	students  int
}

func newLabFixture(t *testing.T, addresses ...string) *labFixture {
	t.Helper()
	db := openTestDatabase(t)
	f := &labFixture{db: db, repo: NewLabRepository(db)}

	f.adminID = insertUser(t, db, "admin@example.edu", "active").ID
	levelID := insertID(t, db,
		`INSERT INTO levels (name, slug, sequence_no, status) VALUES ('Level 1', 'level-1', 1, 'published')`)
	assessmentID := insertID(t, db,
		`INSERT INTO assessments (level_id, title, slug, created_by) VALUES (?, 'Level 1', 'level-1', ?)`,
		levelID, f.adminID)
	f.versionID = insertID(t, db,
		`INSERT INTO assessment_versions
        (assessment_id, version_no, instructions, duration_seconds, pass_score,
         total_points, status, published_at, created_by)
        VALUES (?, 1, 'Find the flags.', 3600, 80, 100, 'published', UTC_TIMESTAMP(6), ?)`,
		assessmentID, f.adminID)

	templateID := insertID(t, db,
		`INSERT INTO lab_templates (name, slug, status, created_by) VALUES ('Scenario 1', 'scenario1', 'active', ?)`,
		f.adminID)
	spec := "image: scenario1\n"
	digest := sha256.Sum256([]byte(spec))
	f.labID = insertID(t, db,
		`INSERT INTO lab_template_versions
        (lab_template_id, version_no, yaml_spec, spec_sha256, status, published_at, created_by)
        VALUES (?, 1, ?, ?, 'published', UTC_TIMESTAMP(6), ?)`,
		templateID, spec, digest[:], f.adminID)

	hostID := insertID(t, db,
		`INSERT INTO docker_hosts (name, endpoint_reference, status, max_instances)
        VALUES ('lab-host', 'unix:///var/run/docker.sock', 'online', 100)`)
	f.poolID = insertID(t, db,
		`INSERT INTO network_pools (docker_host_id, name, cidr) VALUES (?, 'macvlan', '192.168.20.0/24')`,
		hostID)
	for _, address := range addresses {
		f.addAddress(t, address, false)
	}
	return f
}

func (f *labFixture) addAddress(t *testing.T, address string, reserved bool) {
	t.Helper()
	var reason any
	if reserved {
		reason = "gateway"
	}
	exec(t, f.db,
		`INSERT INTO network_addresses (network_pool_id, ip_address, is_reserved, reservation_reason)
        VALUES (?, ?, ?, ?)`,
		f.poolID, netip.MustParseAddr(address).AsSlice(), reserved, reason)
}

// attempt creates a student with an in-progress attempt ending at deadline.
func (f *labFixture) attempt(t *testing.T, deadline time.Time) uint64 {
	t.Helper()
	f.students++
	studentID := insertUser(t, f.db, fmt.Sprintf("student%d@example.edu", f.students), "active").ID
	now := time.Now().UTC()
	grantID := insertID(t, f.db,
		`INSERT INTO assessment_access_grants
        (user_id, assessment_version_id, source, status, eligible_from, eligible_until,
         approved_by, approved_at)
        VALUES (?, ?, 'manual_admin', 'approved', ?, ?, ?, ?)`,
		studentID, f.versionID, now.Add(-time.Hour), now.Add(time.Hour), f.adminID, now)
	return insertID(t, f.db,
		`INSERT INTO attempts
        (access_grant_id, status, started_by, started_at, deadline_at, max_score)
        VALUES (?, 'in_progress', ?, ?, ?, 100)`,
		grantID, studentID, deadline.Add(-time.Hour), deadline)
}

func (f *labFixture) provision(t *testing.T, attemptID uint64) uint64 {
	t.Helper()
	instanceID, err := f.repo.RequestProvision(context.Background(), attemptID, f.labID)
	if err != nil {
		t.Fatalf("RequestProvision() error = %v", err)
	}
	return instanceID
}

func (f *labFixture) claim(t *testing.T) labs.Job {
	t.Helper()
	job, found, err := f.repo.ClaimJob(context.Background(), "test-worker", time.Minute)
	if err != nil || !found {
		t.Fatalf("ClaimJob() = %v, %v; want a job", found, err)
	}
	return job
}

func (f *labFixture) instanceStatus(t *testing.T, instanceID uint64) string {
	t.Helper()
	var status string
	if err := f.db.QueryRow(`SELECT status FROM lab_instances WHERE id = ?`, instanceID).Scan(&status); err != nil {
		t.Fatalf("read instance status: %v", err)
	}
	return status
}

func (f *labFixture) jobStatus(t *testing.T, instanceID uint64, jobType string) string {
	t.Helper()
	var status string
	if err := f.db.QueryRow(
		`SELECT status FROM orchestration_jobs WHERE lab_instance_id = ? AND job_type = ?`,
		instanceID, jobType,
	).Scan(&status); err != nil {
		t.Fatalf("read %s job status: %v", jobType, err)
	}
	return status
}

func (f *labFixture) activeAllocations(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM ip_allocations WHERE released_at IS NULL`).Scan(&n); err != nil {
		t.Fatalf("count allocations: %v", err)
	}
	return n
}

func insertID(t *testing.T, db *sql.DB, query string, args ...any) uint64 {
	t.Helper()
	result, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read inserted id: %v", err)
	}
	return uint64(id)
}

func inAnHour() time.Time { return time.Now().UTC().Add(time.Hour) }

func TestRequestProvisionQueuesLabWithAddress(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182")
	ctx := context.Background()
	instanceID := f.provision(t, f.attempt(t, inAnHour()))

	if got := f.instanceStatus(t, instanceID); got != "queued" {
		t.Fatalf("instance status = %q; want queued", got)
	}
	if got := f.jobStatus(t, instanceID, labs.JobProvision); got != "pending" {
		t.Fatalf("provision job status = %q; want pending", got)
	}

	target, err := f.repo.LoadTarget(ctx, instanceID)
	if err != nil {
		t.Fatalf("LoadTarget() error = %v", err)
	}
	want := labs.Target{
		InstanceID: instanceID,
		Status:     "queued",
		Spec:       labs.Spec{Image: "scenario1"},
		Network:    "macvlan",
		IP:         netip.MustParseAddr("192.168.20.182"),
	}
	if target != want {
		t.Fatalf("LoadTarget() = %+v; want %+v", target, want)
	}
}

func TestRequestProvisionRefusesSecondActiveLab(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182", "192.168.20.183")
	attemptID := f.attempt(t, inAnHour())
	f.provision(t, attemptID)

	_, err := f.repo.RequestProvision(context.Background(), attemptID, f.labID)
	if !errors.Is(err, labs.ErrLabActive) {
		t.Fatalf("RequestProvision() error = %v; want %v", err, labs.ErrLabActive)
	}
}

func TestRequestProvisionSkipsReservedAndAllocatedAddresses(t *testing.T) {
	f := newLabFixture(t)
	f.addAddress(t, "192.168.20.1", true)
	f.addAddress(t, "192.168.20.182", false)

	first := f.provision(t, f.attempt(t, inAnHour()))
	target, err := f.repo.LoadTarget(context.Background(), first)
	if err != nil {
		t.Fatalf("LoadTarget() error = %v", err)
	}
	if target.IP != netip.MustParseAddr("192.168.20.182") {
		t.Fatalf("IP = %s; want the only unreserved address", target.IP)
	}

	_, err = f.repo.RequestProvision(context.Background(), f.attempt(t, inAnHour()), f.labID)
	if !errors.Is(err, labs.ErrNoFreeAddress) {
		t.Fatalf("RequestProvision() error = %v; want %v", err, labs.ErrNoFreeAddress)
	}
}

func TestRequestProvisionConcurrentStartsGetDistinctAddresses(t *testing.T) {
	// A test start provisions a whole room at once.
	addresses := make([]string, 25)
	for i := range addresses {
		addresses[i] = fmt.Sprintf("192.168.20.%d", 182+i)
	}
	f := newLabFixture(t, addresses...)
	attempts := make([]uint64, len(addresses))
	for i := range attempts {
		attempts[i] = f.attempt(t, inAnHour())
	}

	var wg sync.WaitGroup
	errs := make([]error, len(attempts))
	for i, attemptID := range attempts {
		wg.Go(func() {
			_, errs[i] = f.repo.RequestProvision(context.Background(), attemptID, f.labID)
		})
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("attempt %d: RequestProvision() error = %v", i, err)
		}
	}
	var distinct int
	if err := f.db.QueryRow(
		`SELECT COUNT(DISTINCT network_address_id) FROM ip_allocations WHERE released_at IS NULL`,
	).Scan(&distinct); err != nil {
		t.Fatalf("count addresses: %v", err)
	}
	if distinct != len(addresses) {
		t.Fatalf("distinct active addresses = %d; want %d", distinct, len(addresses))
	}
}

func TestLabLifecycleReleasesAddress(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182")
	ctx := context.Background()
	attemptID := f.attempt(t, inAnHour())
	instanceID := f.provision(t, attemptID)

	job := f.claim(t)
	if job.InstanceID != instanceID || job.Type != labs.JobProvision || job.AttemptCount != 1 || job.MaxAttempts != 3 {
		t.Fatalf("claimed %+v; want first attempt of the provision job", job)
	}
	if err := f.repo.MarkProvisioning(ctx, instanceID); err != nil {
		t.Fatalf("MarkProvisioning() error = %v", err)
	}
	if err := f.repo.MarkRunning(ctx, instanceID, "abc123"); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	if err := f.repo.CompleteJob(ctx, job.ID, "succeeded", nil); err != nil {
		t.Fatalf("CompleteJob() error = %v", err)
	}
	if got := f.instanceStatus(t, instanceID); got != "running" {
		t.Fatalf("instance status = %q; want running", got)
	}

	if err := f.repo.RequestStop(ctx, attemptID); err != nil {
		t.Fatalf("RequestStop() error = %v", err)
	}
	if got := f.instanceStatus(t, instanceID); got != "stopping" {
		t.Fatalf("instance status = %q; want stopping", got)
	}
	stop := f.claim(t)
	if stop.Type != labs.JobStop || stop.InstanceID != instanceID {
		t.Fatalf("claimed %+v; want the stop job", stop)
	}
	if err := f.repo.MarkStopped(ctx, instanceID); err != nil {
		t.Fatalf("MarkStopped() error = %v", err)
	}
	if err := f.repo.CompleteJob(ctx, stop.ID, "succeeded", nil); err != nil {
		t.Fatalf("CompleteJob() error = %v", err)
	}

	if got := f.instanceStatus(t, instanceID); got != "stopped" {
		t.Fatalf("instance status = %q; want stopped", got)
	}
	if n := f.activeAllocations(t); n != 0 {
		t.Fatalf("active allocations = %d; want the address released", n)
	}
	// The released address serves the next student.
	f.provision(t, f.attempt(t, inAnHour()))
}

func TestRequestStopOnQueuedLabFinishesWithoutJob(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182")
	attemptID := f.attempt(t, inAnHour())
	instanceID := f.provision(t, attemptID)

	if err := f.repo.RequestStop(context.Background(), attemptID); err != nil {
		t.Fatalf("RequestStop() error = %v", err)
	}
	if got := f.instanceStatus(t, instanceID); got != "stopped" {
		t.Fatalf("instance status = %q; want stopped", got)
	}
	if got := f.jobStatus(t, instanceID, labs.JobProvision); got != "cancelled" {
		t.Fatalf("provision job = %q; want cancelled", got)
	}
	if _, found, _ := f.repo.ClaimJob(context.Background(), "test-worker", time.Minute); found {
		t.Fatal("ClaimJob() found a job; want none, since no container exists")
	}
	if n := f.activeAllocations(t); n != 0 {
		t.Fatalf("active allocations = %d; want 0", n)
	}
	// Repeating the stop is harmless.
	if err := f.repo.RequestStop(context.Background(), attemptID); err != nil {
		t.Fatalf("second RequestStop() error = %v", err)
	}
}

func TestMarkRunningAfterStopRequestedReportsStateChange(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182")
	ctx := context.Background()
	attemptID := f.attempt(t, inAnHour())
	instanceID := f.provision(t, attemptID)
	f.claim(t)
	if err := f.repo.MarkProvisioning(ctx, instanceID); err != nil {
		t.Fatalf("MarkProvisioning() error = %v", err)
	}

	if err := f.repo.RequestStop(ctx, attemptID); err != nil {
		t.Fatalf("RequestStop() error = %v", err)
	}
	err := f.repo.MarkRunning(ctx, instanceID, "abc123")
	if !errors.Is(err, labs.ErrStateChanged) {
		t.Fatalf("MarkRunning() error = %v; want %v", err, labs.ErrStateChanged)
	}
}

func TestRetryJobDelaysNextClaim(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182")
	f.provision(t, f.attempt(t, inAnHour()))
	job := f.claim(t)

	if err := f.repo.RetryJob(context.Background(), job.ID, time.Hour, errors.New("no such image")); err != nil {
		t.Fatalf("RetryJob() error = %v", err)
	}
	if _, found, err := f.repo.ClaimJob(context.Background(), "test-worker", time.Minute); found || err != nil {
		t.Fatalf("ClaimJob() = %v, %v; want nothing until the delay passes", found, err)
	}
}

func TestFailProvisionFailsLabAndReleasesAddress(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182")
	instanceID := f.provision(t, f.attempt(t, inAnHour()))
	job := f.claim(t)

	if err := f.repo.FailProvision(context.Background(), job, errors.New("no such image: scenario1")); err != nil {
		t.Fatalf("FailProvision() error = %v", err)
	}

	var status, code, message string
	if err := f.db.QueryRow(
		`SELECT status, failure_code, failure_message FROM lab_instances WHERE id = ?`, instanceID,
	).Scan(&status, &code, &message); err != nil {
		t.Fatalf("read instance: %v", err)
	}
	if status != "failed" || code != "provision_failed" || message != "no such image: scenario1" {
		t.Fatalf("instance = %q, %q, %q; want failed with the cause", status, code, message)
	}
	if got := f.jobStatus(t, instanceID, labs.JobProvision); got != "failed" {
		t.Fatalf("job = %q; want failed", got)
	}
	if n := f.activeAllocations(t); n != 0 {
		t.Fatalf("active allocations = %d; want 0", n)
	}
}

func TestRequestStopRequeuesFailedStopJob(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182")
	ctx := context.Background()
	attemptID := f.attempt(t, inAnHour())
	instanceID := f.provision(t, attemptID)
	f.claim(t)
	if err := f.repo.MarkProvisioning(ctx, instanceID); err != nil {
		t.Fatalf("MarkProvisioning() error = %v", err)
	}
	if err := f.repo.RequestStop(ctx, attemptID); err != nil {
		t.Fatalf("RequestStop() error = %v", err)
	}
	stop := f.claim(t)
	if err := f.repo.CompleteJob(ctx, stop.ID, "failed", errors.New("docker daemon unreachable")); err != nil {
		t.Fatalf("CompleteJob() error = %v", err)
	}

	if err := f.repo.RequestStop(ctx, attemptID); err != nil {
		t.Fatalf("RequestStop() error = %v", err)
	}
	again := f.claim(t)
	if again.ID != stop.ID || again.AttemptCount != 1 {
		t.Fatalf("claimed %+v; want the stop job reset to its first attempt", again)
	}
}

func TestClaimJobTakesOverExpiredLease(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182")
	ctx := context.Background()
	f.provision(t, f.attempt(t, inAnHour()))
	first := f.claim(t)

	if _, found, _ := f.repo.ClaimJob(ctx, "other-worker", time.Hour); found {
		t.Fatal("ClaimJob() took a job whose lease is still valid")
	}
	time.Sleep(5 * time.Millisecond)
	second, found, err := f.repo.ClaimJob(ctx, "other-worker", time.Millisecond)
	if err != nil || !found || second.ID != first.ID || second.AttemptCount != 2 {
		t.Fatalf("ClaimJob() = %+v, %v, %v; want the same job on its second attempt", second, found, err)
	}
}

func TestFailStaleJobsFailsExhaustedJobs(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182")
	instanceID := f.provision(t, f.attempt(t, inAnHour()))
	exec(t, f.db,
		`UPDATE orchestration_jobs
        SET status = 'running', attempt_count = max_attempts,
            locked_at = UTC_TIMESTAMP(6) - INTERVAL 1 HOUR, locked_by = 'dead-worker'
        WHERE lab_instance_id = ?`,
		instanceID)

	failed, err := f.repo.FailStaleJobs(context.Background(), time.Minute)
	if err != nil || failed != 1 {
		t.Fatalf("FailStaleJobs() = %d, %v; want 1, nil", failed, err)
	}
	if got := f.jobStatus(t, instanceID, labs.JobProvision); got != "failed" {
		t.Fatalf("job = %q; want failed", got)
	}
}

func TestExpiredAttemptsAndLiveInstances(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182", "192.168.20.183", "192.168.20.184")
	ctx := context.Background()
	current := f.attempt(t, inAnHour())
	overdue := f.attempt(t, time.Now().UTC().Add(-time.Minute))
	revoked := f.attempt(t, inAnHour())
	f.provision(t, current)
	f.provision(t, overdue)
	revokedLab := f.provision(t, revoked)
	exec(t, f.db,
		`UPDATE attempts SET status = 'revoked', ended_at = UTC_TIMESTAMP(6),
            termination_reason = 'malpractice' WHERE id = ?`,
		revoked)

	expired, err := f.repo.ExpiredAttempts(ctx)
	if err != nil {
		t.Fatalf("ExpiredAttempts() error = %v", err)
	}
	got := map[uint64]bool{}
	for _, id := range expired {
		got[id] = true
	}
	if len(got) != 2 || !got[overdue] || !got[revoked] {
		t.Fatalf("ExpiredAttempts() = %v; want overdue %d and revoked %d", expired, overdue, revoked)
	}

	// Queued labs have no container yet, so they are not live.
	f.claim(t)
	if err := f.repo.MarkProvisioning(ctx, f.firstInstance(t)); err != nil {
		t.Fatalf("MarkProvisioning() error = %v", err)
	}
	live, err := f.repo.LiveInstances(ctx)
	if err != nil {
		t.Fatalf("LiveInstances() error = %v", err)
	}
	if len(live) != 1 || !live[f.firstInstance(t)] || live[revokedLab] {
		t.Fatalf("LiveInstances() = %v; want only the provisioning lab", live)
	}
}

func (f *labFixture) firstInstance(t *testing.T) uint64 {
	t.Helper()
	var id uint64
	if err := f.db.QueryRow(`SELECT MIN(id) FROM lab_instances`).Scan(&id); err != nil {
		t.Fatalf("read first instance: %v", err)
	}
	return id
}

func TestRequestProvisionRespectsHostCapacity(t *testing.T) {
	f := newLabFixture(t, "192.168.20.182", "192.168.20.183")
	exec(t, f.db, `UPDATE docker_hosts SET max_instances = 1`)
	f.provision(t, f.attempt(t, inAnHour()))

	_, err := f.repo.RequestProvision(context.Background(), f.attempt(t, inAnHour()), f.labID)
	if !errors.Is(err, labs.ErrNoFreeAddress) {
		t.Fatalf("RequestProvision() error = %v; want %v once the host is full", err, labs.ErrNoFreeAddress)
	}
}
