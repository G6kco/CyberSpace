package database

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/G6kco/CyberSpace/internal/assessments"
	"github.com/G6kco/CyberSpace/internal/labs"
)

// AssessmentRepository stores attendance, attempts, dealt questions,
// submissions, and audit entries.
//
// Times are passed in from the service (UTC) rather than read from the
// database clock, so one request sees one consistent "now".
type AssessmentRepository struct {
	db *sql.DB
}

func NewAssessmentRepository(db *sql.DB) *AssessmentRepository {
	return &AssessmentRepository{db: db}
}

// attemptFrom joins an attempt to its student, assessment and level.
const attemptFrom = `
    FROM attempts t
    JOIN assessment_access_grants g ON g.id = t.access_grant_id
    JOIN assessment_versions v ON v.id = g.assessment_version_id
    JOIN assessments a ON a.id = v.assessment_id
    JOIN levels l ON l.id = a.level_id`

// summaryColumns scans with scanSummary.
const summaryColumns = `t.id, l.name, a.title, t.status, t.result, t.score_awarded,
    t.max_score, v.pass_score, t.started_at, t.deadline_at, t.ended_at,
    COALESCE(t.termination_reason, '')`

type rowScanner interface{ Scan(dest ...any) error }

func scanSummary(row rowScanner, extra ...any) (assessments.AttemptSummary, error) {
	var s assessments.AttemptSummary
	var started, deadline, ended sql.NullTime
	dest := append([]any{
		&s.ID, &s.Level, &s.Title, &s.Status, &s.Result, &s.Score,
		&s.MaxScore, &s.PassScore, &started, &deadline, &ended, &s.Reason,
	}, extra...)
	if err := row.Scan(dest...); err != nil {
		return s, err
	}
	s.StartedAt, s.DeadlineAt, s.EndedAt = timePtr(started), timePtr(deadline), timePtr(ended)
	return s, nil
}

func (r *AssessmentRepository) Catalog(ctx context.Context) ([]assessments.Assessment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT l.id, l.name, l.sequence_no, a.id, a.title, v.id,
                v.duration_seconds, v.pass_score, v.total_points, v.instructions
        FROM levels l
        JOIN assessments a ON a.level_id = l.id AND a.status = 'published'
        JOIN assessment_versions v ON v.id = a.current_version_id AND v.status = 'published'
        WHERE l.status = 'published'
        ORDER BY l.sequence_no, a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var catalog []assessments.Assessment
	seen := map[uint64]bool{}
	for rows.Next() {
		var a assessments.Assessment
		if err := rows.Scan(&a.LevelID, &a.LevelName, &a.LevelSequence, &a.AssessmentID, &a.Title,
			&a.VersionID, &a.DurationSeconds, &a.PassScore, &a.TotalPoints, &a.Instructions); err != nil {
			return nil, err
		}
		// One assessment per level: the first published one.
		if seen[a.LevelID] {
			continue
		}
		seen[a.LevelID] = true
		catalog = append(catalog, a)
	}
	return catalog, rows.Err()
}

func (r *AssessmentRepository) StudentState(
	ctx context.Context,
	userID uint64,
	now time.Time,
) (assessments.StudentState, error) {
	state := assessments.StudentState{
		PassedLevels:       map[uint64]bool{},
		LastFailure:        map[uint64]time.Time{},
		AttendanceVersions: map[uint64]bool{},
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+summaryColumns+`, a.level_id`+attemptFrom+`
        WHERE g.user_id = ?
        ORDER BY t.id DESC`,
		userID)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var levelID uint64
		s, err := scanSummary(rows, &levelID)
		if err != nil {
			return state, err
		}
		switch {
		case s.Status == "in_progress" || s.Status == "starting":
			if state.ActiveAttemptID == 0 {
				state.ActiveAttemptID = s.ID
			}
		case s.Status == "evaluated" && s.Result == "passed":
			state.PassedLevels[levelID] = true
		case s.EndedAt != nil:
			if last, ok := state.LastFailure[levelID]; !ok || s.EndedAt.After(last) {
				state.LastFailure[levelID] = *s.EndedAt
			}
		}
		if len(state.History) < 50 {
			state.History = append(state.History, s)
		}
	}
	if err := rows.Err(); err != nil {
		return state, err
	}

	grants, err := r.db.QueryContext(ctx,
		`SELECT assessment_version_id FROM assessment_access_grants
        WHERE user_id = ? AND status = 'approved'
          AND eligible_from <= ? AND eligible_until > ?`,
		userID, now, now)
	if err != nil {
		return state, err
	}
	defer grants.Close()
	for grants.Next() {
		var versionID uint64
		if err := grants.Scan(&versionID); err != nil {
			return state, err
		}
		state.AttendanceVersions[versionID] = true
	}
	return state, grants.Err()
}

const studentColumns = `u.id, u.public_id, u.display_name, u.email,
    COALESCE(sp.register_number, ''), u.status`

func scanStudent(row rowScanner, extra ...any) (assessments.Student, error) {
	var s assessments.Student
	err := row.Scan(append([]any{&s.ID, &s.PublicID, &s.Name, &s.Email, &s.RegisterNumber, &s.Status}, extra...)...)
	return s, err
}

func (r *AssessmentRepository) Students(ctx context.Context, ids []uint64) ([]assessments.Student, error) {
	if len(ids) == 0 {
		return []assessments.Student{}, nil
	}
	placeholders, args := inList(ids)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+studentColumns+`
        FROM users u LEFT JOIN student_profiles sp ON sp.user_id = u.id
        WHERE u.role = 'student' AND u.id IN (`+placeholders+`)
        ORDER BY u.display_name, u.id`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	students := []assessments.Student{}
	for rows.Next() {
		s, err := scanStudent(rows)
		if err != nil {
			return nil, err
		}
		students = append(students, s)
	}
	return students, rows.Err()
}

func (r *AssessmentRepository) StudentByPublicID(ctx context.Context, publicID string) (assessments.Student, error) {
	s, err := scanStudent(r.db.QueryRowContext(ctx,
		`SELECT `+studentColumns+`
        FROM users u LEFT JOIN student_profiles sp ON sp.user_id = u.id
        WHERE u.public_id = ? AND u.role = 'student' AND u.status = 'active'`,
		publicID))
	if errors.Is(err, sql.ErrNoRows) {
		return s, assessments.ErrNotFound
	}
	return s, err
}

func (r *AssessmentRepository) StudentRecords(
	ctx context.Context,
	search string,
	limit int,
) ([]assessments.StudentRecord, error) {
	pattern := "%" + escapeLike(search) + "%"
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+studentColumns+`,
            (SELECT COUNT(DISTINCT a.level_id)`+attemptFrom+`
             WHERE g.user_id = u.id AND t.status = 'evaluated' AND t.result = 'passed')
        FROM users u LEFT JOIN student_profiles sp ON sp.user_id = u.id
        WHERE u.role = 'student'
          AND (? = '' OR u.display_name LIKE ? OR u.email LIKE ? OR sp.register_number LIKE ?)
        ORDER BY u.display_name, u.id
        LIMIT ?`,
		search, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []assessments.StudentRecord{}
	index := map[uint64]int{}
	for rows.Next() {
		var record assessments.StudentRecord
		record.Student, err = scanStudent(rows, &record.LevelsPassed)
		if err != nil {
			return nil, err
		}
		index[record.ID] = len(records)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil || len(records) == 0 {
		return records, err
	}

	ids := make([]uint64, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	placeholders, args := inList(ids)
	last, err := r.db.QueryContext(ctx,
		`SELECT `+summaryColumns+`, g.user_id`+attemptFrom+`
        WHERE g.user_id IN (`+placeholders+`)
          AND t.id = (
              SELECT MAX(t2.id) FROM attempts t2
              JOIN assessment_access_grants g2 ON g2.id = t2.access_grant_id
              WHERE g2.user_id = g.user_id
          )`,
		args...)
	if err != nil {
		return nil, err
	}
	defer last.Close()
	for last.Next() {
		var userID uint64
		s, err := scanSummary(last, &userID)
		if err != nil {
			return nil, err
		}
		records[index[userID]].LastAttempt = &s
	}
	return records, last.Err()
}

func (r *AssessmentRepository) MarkAttendance(
	ctx context.Context,
	adminID, userID, versionID uint64,
	now, until time.Time,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// The user row serializes attendance and starts for one student, so two
	// administrators cannot create two grants or two attempts at once.
	if err := lockUser(ctx, tx, userID); err != nil {
		return err
	}

	var existing uint64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM assessment_access_grants
        WHERE user_id = ? AND assessment_version_id = ? AND status = 'approved'
          AND eligible_from <= ? AND eligible_until > ?
        LIMIT 1`,
		userID, versionID, now, now).Scan(&existing)
	if err == nil {
		return nil // already taken; marking again is harmless
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO assessment_access_grants
        (user_id, assessment_version_id, source, status, eligible_from, eligible_until,
         approved_by, approved_at, created_at)
        VALUES (?, ?, 'manual_admin', 'approved', ?, ?, ?, ?, ?)`,
		userID, versionID, now, until, adminID, now, now,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *AssessmentRepository) CancelAttendance(
	ctx context.Context,
	adminID, userID uint64,
	now time.Time,
	reason string,
) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE assessment_access_grants
        SET status = 'revoked', revoked_by = ?, revoked_at = ?, revocation_reason = ?
        WHERE user_id = ? AND status = 'approved' AND eligible_until > ?`,
		adminID, now, reason, userID, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *AssessmentRepository) AttendanceUsers(ctx context.Context, now time.Time) (map[uint64]uint64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT g.user_id, g.assessment_version_id
        FROM assessment_access_grants g
        JOIN users u ON u.id = g.user_id AND u.role = 'student'
        WHERE g.status = 'approved' AND g.eligible_from <= ? AND g.eligible_until > ?`,
		now, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := map[uint64]uint64{}
	for rows.Next() {
		var userID, versionID uint64
		if err := rows.Scan(&userID, &versionID); err != nil {
			return nil, err
		}
		users[userID] = versionID
	}
	return users, rows.Err()
}

func (r *AssessmentRepository) StartAttempt(
	ctx context.Context,
	req assessments.StartRequest,
) (assessments.Started, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return assessments.Started{}, err
	}
	defer tx.Rollback()

	if err := lockUser(ctx, tx, req.UserID); err != nil {
		return assessments.Started{}, err
	}

	var active int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM attempts t
        JOIN assessment_access_grants g ON g.id = t.access_grant_id
        WHERE g.user_id = ? AND t.status IN ('starting', 'in_progress')`,
		req.UserID).Scan(&active); err != nil {
		return assessments.Started{}, err
	}
	if active > 0 {
		return assessments.Started{}, fmt.Errorf("%w: an attempt is already in progress", assessments.ErrNotEligible)
	}

	var grantID uint64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM assessment_access_grants
        WHERE user_id = ? AND assessment_version_id = ? AND status = 'approved'
          AND eligible_from <= ? AND eligible_until > ?
        ORDER BY id LIMIT 1`,
		req.UserID, req.VersionID, req.Now, req.Now).Scan(&grantID)
	if errors.Is(err, sql.ErrNoRows) {
		return assessments.Started{}, assessments.ErrNoAttendance
	}
	if err != nil {
		return assessments.Started{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE assessment_access_grants SET status = 'consumed', consumed_at = ? WHERE id = ?`,
		req.Now, grantID); err != nil {
		return assessments.Started{}, err
	}

	attemptID, err := insertedID(tx.ExecContext(ctx,
		`INSERT INTO attempts
        (access_grant_id, status, started_by, started_at, deadline_at, max_score, created_at)
        VALUES (?, 'in_progress', ?, ?, ?, ?, ?)`,
		grantID, req.AdminID, req.Now,
		req.Now.Add(time.Duration(req.DurationSeconds)*time.Second), req.MaxScore, req.Now))
	if err != nil {
		return assessments.Started{}, err
	}

	started := assessments.Started{AttemptID: attemptID}
	var codes []string
	for i, pool := range assessments.Pools {
		var questionID uint64
		var code string
		var labID sql.NullInt64
		err := tx.QueryRowContext(ctx,
			`SELECT id, code, lab_template_version_id FROM assessment_questions
            WHERE assessment_version_id = ? AND pool = ? AND parent_question_id IS NULL
            ORDER BY RAND() LIMIT 1`,
			req.VersionID, pool).Scan(&questionID, &code, &labID)
		if errors.Is(err, sql.ErrNoRows) {
			return assessments.Started{}, fmt.Errorf("the assessment has no %s questions", pool)
		}
		if err != nil {
			return assessments.Started{}, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO attempt_questions (attempt_id, question_id, sequence_no, assigned_at)
            VALUES (?, ?, ?, ?)`,
			attemptID, questionID, i+1, req.Now); err != nil {
			return assessments.Started{}, err
		}
		if labID.Valid {
			started.LabTemplateVersionID = uint64(labID.Int64)
		}
		codes = append(codes, code)
	}

	if err := insertEvent(ctx, tx, attemptID, "started", req.AdminID, "", "in_progress",
		map[string]any{"questions": codes}, req.Now); err != nil {
		return assessments.Started{}, err
	}
	return started, tx.Commit()
}

func (r *AssessmentRepository) Attempt(ctx context.Context, attemptID uint64) (assessments.AttemptView, uint64, error) {
	var view assessments.AttemptView
	var owner uint64
	summary, err := scanSummary(r.db.QueryRowContext(ctx,
		`SELECT `+summaryColumns+`, g.user_id`+attemptFrom+` WHERE t.id = ?`,
		attemptID), &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return view, 0, assessments.ErrNotFound
	}
	if err != nil {
		return view, 0, err
	}
	view.AttemptSummary = summary

	dealt, err := r.dealtQuestions(ctx, []uint64{attemptID}, true)
	if err != nil {
		return view, 0, err
	}
	view.Questions = dealt[attemptID]
	if view.Questions == nil {
		view.Questions = []assessments.DealtQuestion{}
	}

	labViews, err := r.labViews(ctx, []uint64{attemptID})
	if err != nil {
		return view, 0, err
	}
	view.Lab = labViews[attemptID]
	return view, owner, nil
}

// dealtQuestions returns each attempt's questions with their score; with
// flags, it also loads each question's prompt and flag slots.
func (r *AssessmentRepository) dealtQuestions(
	ctx context.Context,
	attemptIDs []uint64,
	withFlags bool,
) (map[uint64][]assessments.DealtQuestion, error) {
	result := map[uint64][]assessments.DealtQuestion{}
	if len(attemptIDs) == 0 {
		return result, nil
	}
	placeholders, args := inList(attemptIDs)
	rows, err := r.db.QueryContext(ctx,
		`SELECT aq.attempt_id, q.id, q.code, q.pool, q.difficulty, q.prompt,
                COALESCE(q.pcap_file, ''), COALESCE(q.lab_template_version_id, 0),
                (SELECT COALESCE(SUM(c.points), 0) FROM assessment_questions c
                 WHERE c.parent_question_id = q.id),
                (SELECT COALESCE(SUM(r.best_score), 0) FROM attempt_question_results r
                 JOIN assessment_questions c ON c.id = r.question_id
                 WHERE r.attempt_id = aq.attempt_id AND c.parent_question_id = q.id
                   AND r.is_solved = TRUE)
        FROM attempt_questions aq
        JOIN assessment_questions q ON q.id = aq.question_id
        WHERE aq.attempt_id IN (`+placeholders+`)
        ORDER BY aq.attempt_id, aq.sequence_no`,
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var attemptID uint64
		var q assessments.DealtQuestion
		if err := rows.Scan(&attemptID, &q.ID, &q.Code, &q.Pool, &q.Difficulty, &q.Prompt,
			&q.PcapFile, &q.LabTemplateVersionID, &q.Points, &q.Score); err != nil {
			return nil, err
		}
		if !withFlags {
			q.Prompt = ""
		}
		q.Flags = []assessments.FlagView{}
		result[attemptID] = append(result[attemptID], q)
	}
	if err := rows.Err(); err != nil || !withFlags {
		return result, err
	}

	flags, err := r.db.QueryContext(ctx,
		`SELECT aq.attempt_id, c.parent_question_id, c.id, c.prompt, c.points,
                COALESCE(r.is_solved, FALSE),
                (SELECT COUNT(*) FROM question_submissions s
                 WHERE s.attempt_id = aq.attempt_id AND s.question_id = c.id)
        FROM attempt_questions aq
        JOIN assessment_questions c ON c.parent_question_id = aq.question_id
        LEFT JOIN attempt_question_results r
               ON r.attempt_id = aq.attempt_id AND r.question_id = c.id
        WHERE aq.attempt_id IN (`+placeholders+`)
        ORDER BY aq.attempt_id, aq.sequence_no, c.sequence_no`,
		args...)
	if err != nil {
		return nil, err
	}
	defer flags.Close()
	for flags.Next() {
		var attemptID, parentID uint64
		var f assessments.FlagView
		if err := flags.Scan(&attemptID, &parentID, &f.ID, &f.Prompt, &f.Points, &f.Solved, &f.Submissions); err != nil {
			return nil, err
		}
		questions := result[attemptID]
		for i := range questions {
			if questions[i].ID == parentID {
				questions[i].Flags = append(questions[i].Flags, f)
				break
			}
		}
	}
	return result, flags.Err()
}

// labViews returns the state of each attempt's newest lab.
func (r *AssessmentRepository) labViews(ctx context.Context, attemptIDs []uint64) (map[uint64]*assessments.LabView, error) {
	result := map[uint64]*assessments.LabView{}
	if len(attemptIDs) == 0 {
		return result, nil
	}
	placeholders, args := inList(attemptIDs)
	rows, err := r.db.QueryContext(ctx,
		`SELECT li.attempt_id, li.status, na.ip_address
        FROM lab_instances li
        LEFT JOIN lab_instance_services s
               ON s.lab_instance_id = li.id AND s.service_name = ?
        LEFT JOIN ip_allocations ia ON ia.active_service_id = s.id
        LEFT JOIN network_addresses na ON na.id = ia.network_address_id
        WHERE li.attempt_id IN (`+placeholders+`)
          AND li.generation_no = (
              SELECT MAX(x.generation_no) FROM lab_instances x WHERE x.attempt_id = li.attempt_id
          )`,
		append([]any{labs.TargetService}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var attemptID uint64
		var status string
		var address []byte
		if err := rows.Scan(&attemptID, &status, &address); err != nil {
			return nil, err
		}
		view := &assessments.LabView{Status: status}
		if status == "running" {
			if ip, ok := netip.AddrFromSlice(address); ok {
				view.IP = ip.Unmap().String()
			}
		}
		result[attemptID] = view
	}
	return result, rows.Err()
}

func (r *AssessmentRepository) SubmitAnswer(
	ctx context.Context,
	attemptID, userID, flagID uint64,
	digest [32]byte,
	now time.Time,
) (assessments.SubmitResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return assessments.SubmitResult{}, err
	}
	defer tx.Rollback()

	// Locking the attempt serializes its submissions with each other and
	// with finishing, so a flag cannot be scored after the attempt closed.
	var status string
	var deadline sql.NullTime
	var score float64
	var owner uint64
	err = tx.QueryRowContext(ctx,
		`SELECT t.status, t.deadline_at, t.score_awarded, g.user_id
        FROM attempts t JOIN assessment_access_grants g ON g.id = t.access_grant_id
        WHERE t.id = ?
        FOR UPDATE OF t`,
		attemptID).Scan(&status, &deadline, &score, &owner)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && owner != userID) {
		return assessments.SubmitResult{}, assessments.ErrNotFound
	}
	if err != nil {
		return assessments.SubmitResult{}, err
	}
	if status != "in_progress" || !deadline.Valid || !now.Before(deadline.Time) {
		return assessments.SubmitResult{}, assessments.ErrAttemptClosed
	}

	// The flag must belong to a question dealt to this attempt.
	var points float64
	var maxSubmissions sql.NullInt64
	var expected []byte
	err = tx.QueryRowContext(ctx,
		`SELECT c.points, c.max_submissions, av.expected_hmac
        FROM assessment_questions c
        JOIN attempt_questions aq ON aq.question_id = c.parent_question_id AND aq.attempt_id = ?
        JOIN answer_validators av ON av.question_id = c.id AND av.sequence_no = 1
        WHERE c.id = ?`,
		attemptID, flagID).Scan(&points, &maxSubmissions, &expected)
	if errors.Is(err, sql.ErrNoRows) {
		return assessments.SubmitResult{}, assessments.ErrNotFound
	}
	if err != nil {
		return assessments.SubmitResult{}, err
	}

	var solved bool
	err = tx.QueryRowContext(ctx,
		`SELECT is_solved FROM attempt_question_results WHERE attempt_id = ? AND question_id = ?`,
		attemptID, flagID).Scan(&solved)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return assessments.SubmitResult{}, err
	}
	if solved {
		return assessments.SubmitResult{}, assessments.ErrAlreadySolved
	}

	var count, lastSeq int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(sequence_no), 0) FROM question_submissions
        WHERE attempt_id = ? AND question_id = ?`,
		attemptID, flagID).Scan(&count, &lastSeq); err != nil {
		return assessments.SubmitResult{}, err
	}
	if maxSubmissions.Valid && int64(count) >= maxSubmissions.Int64 {
		return assessments.SubmitResult{}, assessments.ErrTooManySubmissions
	}

	correct := hmac.Equal(expected, digest[:])
	awarded := 0.0
	if correct {
		awarded = points
	}

	submissionID, err := insertedID(tx.ExecContext(ctx,
		`INSERT INTO question_submissions
        (attempt_id, question_id, sequence_no, answer_digest, is_correct,
         points_awarded, evaluated_at, submitted_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		attemptID, flagID, lastSeq+1, digest[:], correct, awarded, now, now))
	if err != nil {
		return assessments.SubmitResult{}, err
	}

	var solvedAt any
	if correct {
		solvedAt = now
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO attempt_question_results
        (attempt_id, question_id, best_score, is_solved, solved_at, last_submission_id, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?) AS new
        ON DUPLICATE KEY UPDATE
            best_score = new.best_score,
            is_solved = new.is_solved,
            solved_at = new.solved_at,
            last_submission_id = new.last_submission_id,
            updated_at = new.updated_at`,
		attemptID, flagID, awarded, correct, solvedAt, submissionID, now); err != nil {
		return assessments.SubmitResult{}, err
	}

	if correct {
		score += awarded
		if _, err := tx.ExecContext(ctx,
			`UPDATE attempts SET score_awarded = ? WHERE id = ?`, score, attemptID); err != nil {
			return assessments.SubmitResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return assessments.SubmitResult{}, err
	}
	return assessments.SubmitResult{Correct: correct, Points: awarded, Score: score}, nil
}

func (r *AssessmentRepository) FinishAttempt(
	ctx context.Context,
	attemptID, actorID uint64,
	mode, reason string,
	now time.Time,
) (assessments.AttemptSummary, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return assessments.AttemptSummary{}, err
	}
	defer tx.Rollback()

	var status string
	var score, passScore float64
	var deadline sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT t.status, t.score_awarded, v.pass_score, t.deadline_at
        FROM attempts t
        JOIN assessment_access_grants g ON g.id = t.access_grant_id
        JOIN assessment_versions v ON v.id = g.assessment_version_id
        WHERE t.id = ?
        FOR UPDATE OF t`,
		attemptID).Scan(&status, &score, &passScore, &deadline)
	if errors.Is(err, sql.ErrNoRows) {
		return assessments.AttemptSummary{}, assessments.ErrNotFound
	}
	if err != nil {
		return assessments.AttemptSummary{}, err
	}
	if status != "in_progress" {
		return assessments.AttemptSummary{}, assessments.ErrAttemptClosed
	}

	var event, newStatus, result string
	var reasonValue any
	if reason != "" {
		reasonValue = reason
	}
	switch mode {
	case assessments.FinishRevoke:
		event, newStatus, result = "revoked", "revoked", "pending"
		if _, err = tx.ExecContext(ctx,
			`UPDATE attempts SET status = 'revoked', ended_at = ?, termination_reason = ? WHERE id = ?`,
			now, reason, attemptID); err != nil {
			return assessments.AttemptSummary{}, err
		}
	case assessments.FinishSubmit, assessments.FinishEnd, assessments.FinishExpire:
		submittedAt := now
		event = map[string]string{
			assessments.FinishSubmit: "submitted",
			assessments.FinishEnd:    "ended",
			assessments.FinishExpire: "expired",
		}[mode]
		if mode == assessments.FinishExpire {
			if !deadline.Valid || deadline.Time.After(now) {
				return assessments.AttemptSummary{}, fmt.Errorf("%w: attempt is not due", assessments.ErrInvalidInput)
			}
			// Scored as handed in at the deadline, whenever the job ran.
			submittedAt = deadline.Time
		}
		newStatus, result = "evaluated", "failed"
		if score >= passScore {
			result = "passed"
		}
		if _, err = tx.ExecContext(ctx,
			`UPDATE attempts
            SET status = 'evaluated', result = ?, submitted_at = ?, ended_at = ?,
                termination_reason = ?
            WHERE id = ?`,
			result, submittedAt, now, reasonValue, attemptID); err != nil {
			return assessments.AttemptSummary{}, err
		}
	default:
		return assessments.AttemptSummary{}, fmt.Errorf("%w: unknown finish mode %q", assessments.ErrInvalidInput, mode)
	}

	if err := insertEvent(ctx, tx, attemptID, event, actorID, "in_progress", newStatus,
		map[string]any{"score": score, "result": result, "reason": reason}, now); err != nil {
		return assessments.AttemptSummary{}, err
	}
	if err := tx.Commit(); err != nil {
		return assessments.AttemptSummary{}, err
	}

	view, _, err := r.Attempt(ctx, attemptID)
	return view.AttemptSummary, err
}

func (r *AssessmentRepository) DueAttempts(ctx context.Context, now time.Time) ([]uint64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM attempts WHERE status = 'in_progress' AND deadline_at <= ?`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *AssessmentRepository) AttemptLab(ctx context.Context, attemptID uint64) (uint64, error) {
	var templateID uint64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(q.lab_template_version_id), 0)
        FROM attempt_questions aq
        JOIN assessment_questions q ON q.id = aq.question_id
        WHERE aq.attempt_id = ? AND q.pool = 'nmap'`,
		attemptID).Scan(&templateID)
	return templateID, err
}

func (r *AssessmentRepository) ListAttempts(
	ctx context.Context,
	active bool,
	limit int,
) ([]assessments.AdminAttempt, error) {
	filter := ""
	if active {
		filter = `WHERE t.status IN ('starting', 'in_progress')`
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+summaryColumns+`, `+studentColumns+attemptFrom+`
        JOIN users u ON u.id = g.user_id
        LEFT JOIN student_profiles sp ON sp.user_id = u.id
        `+filter+`
        ORDER BY t.id DESC
        LIMIT ?`,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []assessments.AdminAttempt{}
	var ids []uint64
	for rows.Next() {
		var item assessments.AdminAttempt
		s := &item.Student
		item.AttemptSummary, err = scanSummary(rows,
			&s.ID, &s.PublicID, &s.Name, &s.Email, &s.RegisterNumber, &s.Status)
		if err != nil {
			return nil, err
		}
		ids = append(ids, item.ID)
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	dealt, err := r.dealtQuestions(ctx, ids, false)
	if err != nil {
		return nil, err
	}
	labViews, err := r.labViews(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Questions = dealt[list[i].ID]
		if list[i].Questions == nil {
			list[i].Questions = []assessments.DealtQuestion{}
		}
		list[i].Lab = labViews[list[i].ID]
	}
	return list, nil
}

func (r *AssessmentRepository) QuestionBank(ctx context.Context) ([]assessments.BankAssessment, error) {
	catalog, err := r.Catalog(ctx)
	if err != nil {
		return nil, err
	}

	bank := make([]assessments.BankAssessment, 0, len(catalog))
	for _, a := range catalog {
		entry := assessments.BankAssessment{Assessment: a, Questions: []assessments.BankQuestion{}}

		rows, err := r.db.QueryContext(ctx,
			`SELECT q.id, q.code, q.pool, q.difficulty, q.prompt,
                    COALESCE(q.pcap_file, ''), COALESCE(ltv.yaml_spec, ''),
                    (SELECT COALESCE(SUM(c.points), 0) FROM assessment_questions c
                     WHERE c.parent_question_id = q.id),
                    (SELECT COUNT(*) FROM attempt_questions aq WHERE aq.question_id = q.id)
            FROM assessment_questions q
            LEFT JOIN lab_template_versions ltv ON ltv.id = q.lab_template_version_id
            WHERE q.assessment_version_id = ? AND q.parent_question_id IS NULL
            ORDER BY q.sequence_no`,
			a.VersionID)
		if err != nil {
			return nil, err
		}
		index := map[uint64]int{}
		for rows.Next() {
			var q assessments.BankQuestion
			var spec string
			if err := rows.Scan(&q.ID, &q.Code, &q.Pool, &q.Difficulty, &q.Prompt,
				&q.PcapFile, &spec, &q.Points, &q.TimesDealt); err != nil {
				rows.Close()
				return nil, err
			}
			if spec != "" {
				if parsed, err := labs.ParseSpec(spec); err == nil {
					q.Image = parsed.Image
				}
			}
			q.FlagPrompts = []string{}
			index[q.ID] = len(entry.Questions)
			entry.Questions = append(entry.Questions, q)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}

		children, err := r.db.QueryContext(ctx,
			`SELECT parent_question_id, prompt FROM assessment_questions
            WHERE assessment_version_id = ? AND parent_question_id IS NOT NULL
            ORDER BY parent_question_id, sequence_no`,
			a.VersionID)
		if err != nil {
			return nil, err
		}
		for children.Next() {
			var parentID uint64
			var prompt string
			if err := children.Scan(&parentID, &prompt); err != nil {
				children.Close()
				return nil, err
			}
			if i, ok := index[parentID]; ok {
				entry.Questions[i].FlagPrompts = append(entry.Questions[i].FlagPrompts, prompt)
			}
		}
		children.Close()
		if err := children.Err(); err != nil {
			return nil, err
		}
		bank = append(bank, entry)
	}
	return bank, nil
}

func (r *AssessmentRepository) Audit(ctx context.Context, entry assessments.AuditEntry) error {
	var actor, after, ip, agent any
	if entry.Actor.UserID != 0 {
		actor = entry.Actor.UserID
	}
	if entry.After != nil {
		data, err := json.Marshal(entry.After)
		if err != nil {
			return err
		}
		after = data
	}
	if v4 := entry.Actor.IP.To4(); v4 != nil {
		ip = []byte(v4)
	} else if v6 := entry.Actor.IP.To16(); v6 != nil {
		ip = []byte(v6)
	}
	if entry.Actor.UserAgent != "" {
		agent = truncate(entry.Actor.UserAgent, 512)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_logs
        (actor_user_id, action, outcome, entity_type, entity_id, after_data,
         ip_address, user_agent, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6))`,
		actor, entry.Action, entry.Outcome, entry.EntityType, entry.EntityID, after, ip, agent)
	return err
}

func lockUser(ctx context.Context, tx *sql.Tx, userID uint64) error {
	var id uint64
	err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return assessments.ErrNotFound
	}
	return err
}

func insertEvent(
	ctx context.Context,
	tx *sql.Tx,
	attemptID uint64,
	eventType string,
	actorID uint64,
	from, to string,
	details any,
	now time.Time,
) error {
	var actor, fromStatus, toStatus any
	if actorID != 0 {
		actor = actorID
	}
	if from != "" {
		fromStatus = from
	}
	if to != "" {
		toStatus = to
	}
	data, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO attempt_events
        (attempt_id, event_type, actor_user_id, from_status, to_status, details_json, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		attemptID, eventType, actor, fromStatus, toStatus, data, now)
	return err
}

func inList(ids []uint64) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

// escapeLike makes s match literally inside a LIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}
