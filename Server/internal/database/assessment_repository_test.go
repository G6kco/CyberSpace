package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/assessments"
	"github.com/G6kco/CyberSpace/internal/evaluation"
	"github.com/G6kco/CyberSpace/internal/labs"
)

var assessmentKey = bytes.Repeat([]byte("a"), evaluation.MinKeyLength)

// fakeLabs records lab requests instead of queueing containers.
type fakeLabs struct {
	mu         sync.Mutex
	provisions map[uint64]uint64 // attempt -> lab template version
	stops      []uint64
	provErr    error
}

func (f *fakeLabs) RequestProvision(_ context.Context, attemptID, templateID uint64) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.provErr != nil {
		return 0, f.provErr
	}
	f.provisions[attemptID] = templateID
	return 1, nil
}

func (f *fakeLabs) RequestStop(_ context.Context, attemptID uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops = append(f.stops, attemptID)
	return nil
}

// flowFixture is a Level 1 bank of two Wireshark questions and one Nmap
// question, an administrator, and students.
type flowFixture struct {
	db      *sql.DB
	service *assessments.Service
	labs    *fakeLabs
	admin   assessments.Actor
	labID   uint64
	pcapDir string
	codes   map[string]uint64 // question code -> id
}

func newFlowFixture(t *testing.T) *flowFixture {
	t.Helper()
	db := openTestDatabase(t)
	f := &flowFixture{db: db, labs: &fakeLabs{provisions: map[uint64]uint64{}}, codes: map[string]uint64{}}

	f.admin = assessments.Actor{UserID: insertUserWithRole(t, db, "admin@example.edu", "admin")}
	levelID := insertID(t, db,
		`INSERT INTO levels (name, slug, sequence_no, status) VALUES ('Level 1', 'level-1', 1, 'published')`)
	assessmentID := insertID(t, db,
		`INSERT INTO assessments (level_id, title, slug, created_by) VALUES (?, 'Level 1 Practical', 'level-1-practical', ?)`,
		levelID, f.admin.UserID)
	versionID := insertID(t, db,
		`INSERT INTO assessment_versions
        (assessment_id, version_no, instructions, duration_seconds, pass_score,
         total_points, status, published_at, created_by)
        VALUES (?, 1, 'Find the flags.', 3600, 80, 100, 'published', UTC_TIMESTAMP(6), ?)`,
		assessmentID, f.admin.UserID)
	exec(t, db, `UPDATE assessments SET status = 'published', current_version_id = ? WHERE id = ?`,
		versionID, assessmentID)

	templateID := insertID(t, db,
		`INSERT INTO lab_templates (name, slug, status, created_by) VALUES ('scenario1', 'scenario1', 'active', ?)`,
		f.admin.UserID)
	spec := "image: scenario1\n"
	digest := sha256.Sum256([]byte(spec))
	f.labID = insertID(t, db,
		`INSERT INTO lab_template_versions
        (lab_template_id, version_no, yaml_spec, spec_sha256, status, published_at, created_by)
        VALUES (?, 1, ?, ?, 'published', UTC_TIMESTAMP(6), ?)`,
		templateID, spec, digest[:], f.admin.UserID)

	type flag struct {
		code, answer string
		points       int
	}
	group := func(seq int, code, pool, difficulty string, pcap, lab any, flags ...flag) {
		id := insertID(t, db,
			`INSERT INTO assessment_questions
            (assessment_version_id, code, prompt, answer_type, pool, difficulty,
             lab_template_version_id, pcap_file, points, sequence_no)
            VALUES (?, ?, ?, 'group', ?, ?, ?, ?, 0, ?)`,
			versionID, code, "Prompt "+code, pool, difficulty, lab, pcap, seq)
		f.codes[code] = id
		for i, fl := range flags {
			childID := insertID(t, db,
				`INSERT INTO assessment_questions
                (assessment_version_id, parent_question_id, code, prompt, answer_type, points, sequence_no)
                VALUES (?, ?, ?, ?, 'flag', ?, ?)`,
				versionID, id, fl.code, "Prompt "+fl.code, fl.points, i+1)
			d := evaluation.Digest(assessmentKey, fl.answer)
			exec(t, db,
				`INSERT INTO answer_validators (question_id, validator_type, expected_hmac)
                VALUES (?, 'case_insensitive_hmac', ?)`, childID, d[:])
			f.codes[fl.code] = childID
		}
	}
	group(1, "wireshark-01", "wireshark", "easy", "question1.pcap", nil,
		flag{"wireshark-01-flag", "D935e3", 10}, flag{"wireshark-01-q1", "41.133", 40})
	group(2, "wireshark-02", "wireshark", "hard", "scenario6.pcap", nil,
		flag{"wireshark-02-flag", "BITSATHY", 50})
	group(3, "nmap-01", "nmap", "easy", nil, f.labID,
		flag{"nmap-01-flag", "f7f4d1", 50})

	f.pcapDir = t.TempDir()
	for _, name := range []string{"question1.pcap", "scenario6.pcap"} {
		if err := os.WriteFile(filepath.Join(f.pcapDir, name), []byte("pcap"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	service, err := assessments.NewService(NewAssessmentRepository(db), f.labs, zap.NewNop(),
		assessments.Config{AnswerKey: assessmentKey, PcapDir: f.pcapDir, SubmissionsPerMinute: 1000})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	f.service = service
	return f
}

var studentSequence int

func insertUserWithRole(t *testing.T, db *sql.DB, email, role string) uint64 {
	t.Helper()
	studentSequence++
	return insertID(t, db,
		`INSERT INTO users (public_id, email, display_name, role, status) VALUES (?, ?, ?, ?, 'active')`,
		fmt.Sprintf("01FLOWUSER%016d", studentSequence), email, "User "+email, role)
}

// student creates a student and returns its ID and public ID.
func (f *flowFixture) student(t *testing.T, name string) (uint64, string) {
	t.Helper()
	id := insertUserWithRole(t, f.db, name+"@example.edu", "student")
	exec(t, f.db,
		`INSERT INTO student_profiles (user_id, register_number, department, programme, graduation_year)
        VALUES (?, ?, 'CSE', 'BE', 2027)`, id, "REG-"+name)
	var publicID string
	if err := f.db.QueryRow(`SELECT public_id FROM users WHERE id = ?`, id).Scan(&publicID); err != nil {
		t.Fatal(err)
	}
	return id, publicID
}

// started brings a student through the portal, attendance and start.
func (f *flowFixture) started(t *testing.T, name string) (uint64, uint64) {
	t.Helper()
	ctx := context.Background()
	userID, publicID := f.student(t, name)
	if _, err := f.service.Heartbeat(ctx, userID); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if r := f.service.MarkAttendance(ctx, f.admin, []string{publicID}); !r[0].OK {
		t.Fatalf("MarkAttendance() = %+v", r)
	}
	r := f.service.Start(ctx, f.admin, []string{publicID})
	if !r[0].OK || r[0].AttemptID == nil {
		t.Fatalf("Start() = %+v", r)
	}
	return userID, *r[0].AttemptID
}

// flagFor returns the flag of the dealt question in pool.
func (f *flowFixture) dealt(t *testing.T, userID, attemptID uint64, pool string) assessments.DealtQuestion {
	t.Helper()
	view, err := f.service.Attempt(context.Background(), userID, attemptID)
	if err != nil {
		t.Fatalf("Attempt() error = %v", err)
	}
	for _, q := range view.Questions {
		if q.Pool == pool {
			return q
		}
	}
	t.Fatalf("no %s question dealt", pool)
	return assessments.DealtQuestion{}
}

var answers = map[string]string{
	"wireshark-01-flag": "d935E3", "wireshark-01-q1": "41.133",
	"wireshark-02-flag": "bitsathy", "nmap-01-flag": "F7F4D1",
}

func (f *flowFixture) solveAll(t *testing.T, userID, attemptID uint64) {
	t.Helper()
	for _, pool := range assessments.Pools {
		for _, flag := range f.dealt(t, userID, attemptID, pool).Flags {
			code := f.codeOf(flag.ID)
			result, err := f.service.Submit(context.Background(), userID, attemptID, flag.ID, answers[code])
			if err != nil || !result.Correct {
				t.Fatalf("Submit(%s) = %+v, %v; want correct", code, result, err)
			}
		}
	}
}

func (f *flowFixture) codeOf(id uint64) string {
	for code, qid := range f.codes {
		if qid == id {
			return code
		}
	}
	return ""
}

func TestOverviewOfNewStudentIsEligible(t *testing.T) {
	f := newFlowFixture(t)
	userID, _ := f.student(t, "asha")

	overview, err := f.service.Overview(context.Background(), userID)
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if overview.Status != assessments.StatusEligible || overview.Assessment == nil ||
		overview.Assessment.LevelName != "Level 1" || overview.Assessment.PassScore != 80 ||
		overview.Assessment.DurationSeconds != 3600 || overview.AttendanceMarked || overview.InLobby {
		t.Fatalf("Overview() = %+v", overview)
	}
}

func TestAttendanceRequiresPresenceAndStartRequiresAttendance(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, publicID := f.student(t, "ben")

	if r := f.service.MarkAttendance(ctx, f.admin, []string{publicID}); r[0].OK {
		t.Fatalf("MarkAttendance() for a student not in the portal = %+v; want refused", r)
	}
	if r := f.service.Start(ctx, f.admin, []string{publicID}); r[0].OK {
		t.Fatalf("Start() without attendance = %+v; want refused", r)
	}

	if _, err := f.service.Heartbeat(ctx, userID); err != nil {
		t.Fatal(err)
	}
	lobby, err := f.service.Lobby(ctx)
	if err != nil || len(lobby) != 1 || !lobby[0].Online || lobby[0].AttendanceMarked || lobby[0].RegisterNumber != "REG-ben" {
		t.Fatalf("Lobby() = %+v, %v; want ben online without attendance", lobby, err)
	}

	if r := f.service.MarkAttendance(ctx, f.admin, []string{publicID, publicID}); len(r) != 1 || !r[0].OK {
		t.Fatalf("MarkAttendance() = %+v; want one success", r)
	}
	// Marking again is harmless and creates no second grant.
	f.service.MarkAttendance(ctx, f.admin, []string{publicID})
	var grants int
	_ = f.db.QueryRow(`SELECT COUNT(*) FROM assessment_access_grants WHERE user_id = ?`, userID).Scan(&grants)
	if grants != 1 {
		t.Fatalf("grants = %d; want 1", grants)
	}

	overview, _ := f.service.Overview(ctx, userID)
	if !overview.AttendanceMarked {
		t.Fatal("Overview() does not report attendance")
	}

	if _, err := f.service.CancelAttendance(ctx, f.admin, publicID); err != nil {
		t.Fatal(err)
	}
	if r := f.service.Start(ctx, f.admin, []string{publicID}); r[0].OK {
		t.Fatalf("Start() after attendance was withdrawn = %+v; want refused", r)
	}
}

func TestStartDealsOneQuestionPerPoolAndQueuesLab(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, attemptID := f.started(t, "chen")

	view, err := f.service.Attempt(ctx, userID, attemptID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "in_progress" || len(view.Questions) != 2 ||
		view.Questions[0].Pool != "wireshark" || view.Questions[1].Pool != "nmap" {
		t.Fatalf("Attempt() = %+v; want one Wireshark and one Nmap question", view)
	}
	if view.DeadlineAt == nil || view.StartedAt == nil || view.DeadlineAt.Sub(*view.StartedAt) != time.Hour {
		t.Fatalf("deadline = %v, started = %v; want one hour", view.DeadlineAt, view.StartedAt)
	}
	for _, q := range view.Questions {
		if q.Points != 50 {
			t.Errorf("%s points = %v; want 50", q.Code, q.Points)
		}
	}
	if f.labs.provisions[attemptID] != f.labID {
		t.Fatalf("lab requests = %v; want attempt %d on lab %d", f.labs.provisions, attemptID, f.labID)
	}

	overview, _ := f.service.Overview(ctx, userID)
	if overview.Status != assessments.StatusInProgress || overview.ActiveAttemptID == nil || *overview.ActiveAttemptID != attemptID {
		t.Fatalf("Overview() = %+v; want in progress", overview)
	}
	// The grant was consumed: no second attempt can start from it.
	_, publicID := "", ""
	_ = f.db.QueryRow(`SELECT public_id FROM users WHERE id = ?`, userID).Scan(&publicID)
	if r := f.service.Start(ctx, f.admin, []string{publicID}); r[0].OK {
		t.Fatalf("second Start() = %+v; want refused", r)
	}
}

func TestStudentsCannotSeeOrAnswerOthersAttempts(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	ownerID, attemptID := f.started(t, "dev")
	otherID, _ := f.student(t, "eve")
	flag := f.dealt(t, ownerID, attemptID, "nmap").Flags[0]

	if _, err := f.service.Attempt(ctx, otherID, attemptID); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("Attempt() by another student error = %v; want ErrNotFound", err)
	}
	if _, err := f.service.Submit(ctx, otherID, attemptID, flag.ID, "f7f4d1"); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("Submit() by another student error = %v; want ErrNotFound", err)
	}
	if _, err := f.service.Finish(ctx, otherID, attemptID); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("Finish() by another student error = %v; want ErrNotFound", err)
	}
	q := f.dealt(t, ownerID, attemptID, "wireshark")
	if _, _, err := f.service.Pcap(ctx, otherID, attemptID, q.ID); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("Pcap() by another student error = %v; want ErrNotFound", err)
	}
}

func TestSubmitScoresEachFlagOnce(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, attemptID := f.started(t, "fay")
	flag := f.dealt(t, userID, attemptID, "nmap").Flags[0]

	wrong, err := f.service.Submit(ctx, userID, attemptID, flag.ID, "nope")
	if err != nil || wrong.Correct || wrong.Score != 0 {
		t.Fatalf("Submit(wrong) = %+v, %v", wrong, err)
	}
	right, err := f.service.Submit(ctx, userID, attemptID, flag.ID, "  F7F4D1 ")
	if err != nil || !right.Correct || right.Points != 50 || right.Score != 50 {
		t.Fatalf("Submit(right) = %+v, %v; want 50 points", right, err)
	}
	if _, err := f.service.Submit(ctx, userID, attemptID, flag.ID, "f7f4d1"); !errors.Is(err, assessments.ErrAlreadySolved) {
		t.Fatalf("Submit(again) error = %v; want ErrAlreadySolved", err)
	}

	// A flag of a question not dealt to this attempt is not answerable.
	notDealt := f.codes["wireshark-01-flag"]
	if f.dealt(t, userID, attemptID, "wireshark").Code == "wireshark-01" {
		notDealt = f.codes["wireshark-02-flag"]
	}
	if _, err := f.service.Submit(ctx, userID, attemptID, notDealt, "x"); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("Submit(undealt flag) error = %v; want ErrNotFound", err)
	}
	if _, err := f.service.Submit(ctx, userID, attemptID, flag.ID, ""); !errors.Is(err, assessments.ErrInvalidInput) {
		t.Fatalf("Submit(empty) error = %v; want ErrInvalidInput", err)
	}

	view, _ := f.service.Attempt(ctx, userID, attemptID)
	nmap := view.Questions[1]
	if view.Score != 50 || nmap.Score != 50 || !nmap.Flags[0].Solved || nmap.Flags[0].Submissions != 2 {
		t.Fatalf("Attempt() after submissions = %+v", view)
	}
	var stored int
	_ = f.db.QueryRow(`SELECT COUNT(*) FROM question_submissions WHERE attempt_id = ?`, attemptID).Scan(&stored)
	if stored != 2 {
		t.Fatalf("stored submissions = %d; want 2", stored)
	}
}

func TestConcurrentCorrectSubmissionsScoreOnce(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, attemptID := f.started(t, "gil")
	flag := f.dealt(t, userID, attemptID, "nmap").Flags[0]

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { _, _ = f.service.Submit(ctx, userID, attemptID, flag.ID, "f7f4d1") })
	}
	wg.Wait()

	view, _ := f.service.Attempt(ctx, userID, attemptID)
	if view.Score != 50 {
		t.Fatalf("score = %v after concurrent correct submissions; want 50", view.Score)
	}
}

func TestFinishFailingAttemptStartsCooldown(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, attemptID := f.started(t, "hal")

	summary, err := f.service.Finish(ctx, userID, attemptID)
	if err != nil || summary.Status != "evaluated" || summary.Result != "failed" {
		t.Fatalf("Finish() = %+v, %v; want evaluated and failed", summary, err)
	}
	if len(f.labs.stops) != 1 || f.labs.stops[0] != attemptID {
		t.Fatalf("lab stops = %v; want the attempt's lab stopped", f.labs.stops)
	}
	if _, err := f.service.Finish(ctx, userID, attemptID); !errors.Is(err, assessments.ErrAttemptClosed) {
		t.Fatalf("second Finish() error = %v; want ErrAttemptClosed", err)
	}

	view, _ := f.service.Attempt(ctx, userID, attemptID)
	if view.Questions != nil || view.Lab != nil {
		t.Fatalf("finished attempt still shows questions or lab: %+v", view)
	}

	overview, _ := f.service.Overview(ctx, userID)
	if overview.Status != assessments.StatusCooldown || overview.CooldownUntil == nil ||
		overview.CooldownUntil.Sub(*summary.EndedAt) != 24*time.Hour {
		t.Fatalf("Overview() = %+v; want a 24-hour cooldown", overview)
	}
	// A student in cooldown is not listed in the portal.
	if overview, _ := f.service.Heartbeat(ctx, userID); overview.InLobby {
		t.Fatal("Heartbeat() admitted a student in cooldown")
	}
}

func TestPassingAttemptCompletesLevel(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, attemptID := f.started(t, "ivy")
	f.solveAll(t, userID, attemptID)

	summary, err := f.service.Finish(ctx, userID, attemptID)
	if err != nil || summary.Result != "passed" || summary.Score != 100 {
		t.Fatalf("Finish() = %+v, %v; want passed with 100", summary, err)
	}
	overview, _ := f.service.Overview(ctx, userID)
	if overview.Status != assessments.StatusCompleted {
		t.Fatalf("Overview() status = %s; want completed (Level 1 is the only level)", overview.Status)
	}
}

func TestExpireDueScoresAtDeadline(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, attemptID := f.started(t, "jo")
	f.solveAll(t, userID, attemptID)
	exec(t, f.db,
		`UPDATE attempts SET started_at = UTC_TIMESTAMP(6) - INTERVAL 2 HOUR,
            deadline_at = UTC_TIMESTAMP(6) - INTERVAL 1 HOUR WHERE id = ?`, attemptID)

	flag := f.dealt(t, userID, attemptID, "nmap").Flags[0]
	if _, err := f.service.Submit(ctx, userID, attemptID, flag.ID, "x"); !errors.Is(err, assessments.ErrAttemptClosed) {
		t.Fatalf("Submit() after deadline error = %v; want ErrAttemptClosed", err)
	}

	f.service.ExpireDue(ctx)

	var status, result string
	var submitted, deadline time.Time
	if err := f.db.QueryRow(
		`SELECT status, result, submitted_at, deadline_at FROM attempts WHERE id = ?`, attemptID,
	).Scan(&status, &result, &submitted, &deadline); err != nil {
		t.Fatal(err)
	}
	if status != "evaluated" || result != "passed" || !submitted.Equal(deadline) {
		t.Fatalf("expired attempt = %s/%s submitted %v deadline %v; want scored at the deadline", status, result, submitted, deadline)
	}
	if len(f.labs.stops) != 1 {
		t.Fatalf("lab stops = %v; want 1", f.labs.stops)
	}
}

func TestRevokeRequiresReasonAndAudits(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, attemptID := f.started(t, "kim")

	if _, err := f.service.Revoke(ctx, f.admin, attemptID, "  "); !errors.Is(err, assessments.ErrInvalidInput) {
		t.Fatalf("Revoke() without reason error = %v; want ErrInvalidInput", err)
	}
	summary, err := f.service.Revoke(ctx, f.admin, attemptID, "Phone use")
	if err != nil || summary.Status != "revoked" || summary.Reason != "Phone use" || summary.Result != "pending" {
		t.Fatalf("Revoke() = %+v, %v", summary, err)
	}
	overview, _ := f.service.Overview(ctx, userID)
	if overview.Status != assessments.StatusCooldown {
		t.Fatalf("status after revoke = %s; want cooldown", overview.Status)
	}

	var audits int
	_ = f.db.QueryRow(
		`SELECT COUNT(*) FROM audit_logs WHERE action = 'attempt.revoke' AND actor_user_id = ? AND outcome = 'success'`,
		f.admin.UserID).Scan(&audits)
	if audits != 1 {
		t.Fatalf("successful revoke audits = %d; want 1", audits)
	}
	var events int
	_ = f.db.QueryRow(`SELECT COUNT(*) FROM attempt_events WHERE attempt_id = ?`, attemptID).Scan(&events)
	if events != 2 { // started, revoked
		t.Fatalf("attempt events = %d; want 2", events)
	}
}

func TestPcapServedOnlyForDealtQuestionDuringAttempt(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, attemptID := f.started(t, "lee")
	wireshark := f.dealt(t, userID, attemptID, "wireshark")
	nmap := f.dealt(t, userID, attemptID, "nmap")

	path, name, err := f.service.Pcap(ctx, userID, attemptID, wireshark.ID)
	if err != nil || filepath.Dir(path) != f.pcapDir || (name != "question1.pcap" && name != "scenario6.pcap") {
		t.Fatalf("Pcap() = %q, %q, %v", path, name, err)
	}
	if _, _, err := f.service.Pcap(ctx, userID, attemptID, nmap.ID); !errors.Is(err, assessments.ErrNotFound) {
		t.Fatalf("Pcap(nmap question) error = %v; want ErrNotFound", err)
	}

	if _, err := f.service.Finish(ctx, userID, attemptID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.service.Pcap(ctx, userID, attemptID, wireshark.ID); !errors.Is(err, assessments.ErrAttemptClosed) {
		t.Fatalf("Pcap() after finishing error = %v; want ErrAttemptClosed", err)
	}
}

func TestConcurrentStartsCreateOneAttempt(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, publicID := f.student(t, "max")
	_, _ = f.service.Heartbeat(ctx, userID)
	f.service.MarkAttendance(ctx, f.admin, []string{publicID})

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() { f.service.Start(ctx, f.admin, []string{publicID}) })
	}
	wg.Wait()

	var attempts int
	_ = f.db.QueryRow(`SELECT COUNT(*) FROM attempts`).Scan(&attempts)
	if attempts != 1 {
		t.Fatalf("attempts = %d after concurrent starts; want 1", attempts)
	}
}

func TestRestartLabAndAdminViews(t *testing.T) {
	f := newFlowFixture(t)
	ctx := context.Background()
	userID, attemptID := f.started(t, "nia")

	f.labs.provErr = labs.ErrLabActive
	if err := f.service.RestartLab(ctx, f.admin, attemptID); !errors.Is(err, assessments.ErrLabActive) {
		t.Fatalf("RestartLab() while active error = %v; want ErrLabActive", err)
	}
	f.labs.provErr = nil
	if err := f.service.RestartLab(ctx, f.admin, attemptID); err != nil {
		t.Fatalf("RestartLab() error = %v", err)
	}

	active, err := f.service.Attempts(ctx, true)
	if err != nil || len(active) != 1 || active[0].Student.RegisterNumber != "REG-nia" || len(active[0].Questions) != 2 {
		t.Fatalf("Attempts(active) = %+v, %v", active, err)
	}
	if active[0].Questions[0].Prompt != "" {
		t.Fatal("monitoring list carries question prompts; want summaries only")
	}

	bank, err := f.service.QuestionBank(ctx)
	if err != nil || len(bank) != 1 || len(bank[0].Questions) != 3 {
		t.Fatalf("QuestionBank() = %+v, %v", bank, err)
	}
	for _, q := range bank[0].Questions {
		switch q.Pool {
		case "wireshark":
			if !q.PcapPresent {
				t.Errorf("%s pcap not reported present", q.Code)
			}
		case "nmap":
			if q.Image != "scenario1" {
				t.Errorf("%s image = %q", q.Code, q.Image)
			}
		}
	}

	if _, err := f.service.End(ctx, f.admin, attemptID); err != nil {
		t.Fatal(err)
	}
	records, err := f.service.StudentRecords(ctx, "nia")
	if err != nil || len(records) != 1 || records[0].LastAttempt == nil ||
		records[0].LastAttempt.ID != attemptID || records[0].CooldownUntil == nil {
		t.Fatalf("StudentRecords() = %+v, %v", records, err)
	}
	_ = userID
}
