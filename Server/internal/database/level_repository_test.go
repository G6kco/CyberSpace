package database

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/assessments"
	"github.com/G6kco/CyberSpace/internal/levels"
)

// builderFixture is an empty database with an administrator, the level
// builder, and the assessment service sitting what it builds.
type builderFixture struct {
	flow     *flowFixture
	levels   *levels.Service
	filesDir string
}

func newBuilderFixture(t *testing.T, maxFileBytes int64) *builderFixture {
	t.Helper()
	db := openTestDatabase(t)
	flow := &flowFixture{db: db, labs: &fakeLabs{provisions: map[uint64]uint64{}}, codes: map[string]uint64{}}
	flow.admin = assessments.Actor{UserID: insertUserWithRole(t, db, "builder@example.edu", "admin")}

	filesDir := t.TempDir()
	service, err := assessments.NewService(NewAssessmentRepository(db), flow.labs, zap.NewNop(),
		assessments.Config{AnswerKey: assessmentKey, FilesDir: filesDir, SubmissionsPerMinute: 1000})
	if err != nil {
		t.Fatal(err)
	}
	flow.service = service

	builder, err := levels.NewService(NewLevelRepository(db), NewAssessmentRepository(db), zap.NewNop(),
		levels.Config{AnswerKey: assessmentKey, FilesDir: filesDir, MaxFileBytes: maxFileBytes})
	if err != nil {
		t.Fatal(err)
	}
	return &builderFixture{flow: flow, levels: builder, filesDir: filesDir}
}

func (f *builderFixture) level(t *testing.T, perAttempt int, passPercent float64) uint64 {
	t.Helper()
	id, err := f.levels.Create(context.Background(), f.flow.admin, levels.LevelInput{
		Name: "Level 2", Title: "Web scenarios", Instructions: "Each scenario has its own target.",
		DurationMinutes: 90, QuestionsPerAttempt: perAttempt, PassPercent: passPercent,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return id
}

func (f *builderFixture) question(t *testing.T, levelID uint64, image string, answers ...levels.AnswerInput) uint64 {
	t.Helper()
	id, err := f.levels.CreateQuestion(context.Background(), f.flow.admin, levelID, levels.QuestionInput{
		Prompt: "Break into " + image, Image: image, Answers: answers,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() error = %v", err)
	}
	return id
}

func answer(prompt, value string, marks float64) levels.AnswerInput {
	return levels.AnswerInput{Prompt: prompt, Answer: value, Marks: marks}
}

func TestBuiltLevelIsDealtScoredAndLocked(t *testing.T) {
	f := newBuilderFixture(t, 0)
	ctx := context.Background()
	levelID := f.level(t, 2, 60)

	web := f.question(t, levelID, "scenario2-web", answer("Admin password", "Summer2024", 20), answer("Flag", "FLAG{web}", 5))
	// Publishing needs at least as many questions as each student is dealt.
	if err := f.levels.Publish(ctx, f.flow.admin, levelID); !errors.Is(err, levels.ErrNotReady) {
		t.Fatalf("Publish() with 1 question for 2 dealt = %v; want ErrNotReady", err)
	}
	ssh := f.question(t, levelID, "scenario2-ssh", answer("Flag", "flag{ssh}", 15))
	logs := f.question(t, levelID, "", answer("Attacker IP", "10.0.0.7", 40))
	if _, err := f.levels.UploadFile(ctx, f.flow.admin, levelID, logs, "access.log", strings.NewReader("GET /admin")); err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}

	level, err := f.levels.Get(ctx, levelID)
	if err != nil || !level.Built || level.Status != levels.StatusDraft || len(level.Questions) != 3 {
		t.Fatalf("Get() = %+v, %v; want a draft with 3 questions", level, err)
	}
	if q := level.Questions[0]; q.ID != web || q.Image != "scenario2-web" || q.Marks != 25 || len(q.Answers) != 2 {
		t.Fatalf("first question = %+v; want the web scenario worth 25", q)
	}
	if files := level.Questions[2].Files; len(files) != 1 || files[0].Name != "access.log" || !files[0].Present {
		t.Fatalf("log question files = %+v; want access.log present", files)
	}

	if err := f.levels.Publish(ctx, f.flow.admin, levelID); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	// Published: the best two questions are worth 40 + 25.
	var total, passScore float64
	if err := f.flow.db.QueryRow(
		`SELECT v.total_points, v.pass_score FROM assessment_versions v
        JOIN assessments a ON a.id = v.assessment_id WHERE a.level_id = ?`, levelID,
	).Scan(&total, &passScore); err != nil {
		t.Fatal(err)
	}
	if total != 65 || passScore != 39 {
		t.Fatalf("published total, pass = %v, %v; want 65, 39", total, passScore)
	}
	if err := f.levels.Update(ctx, f.flow.admin, levelID, levels.LevelInput{
		Name: "Level 2", DurationMinutes: 60, QuestionsPerAttempt: 1, PassPercent: 50,
	}); !errors.Is(err, levels.ErrPublished) {
		t.Fatalf("Update() of a published level = %v; want ErrPublished", err)
	}

	// A student sits it: two of the three questions, a lab for each one
	// with an image, and a pass mark of 60% of what was dealt.
	userID, attemptID := f.flow.started(t, "ola")
	view, err := f.flow.service.Attempt(ctx, userID, attemptID)
	if err != nil || len(view.Questions) != 2 {
		t.Fatalf("Attempt() = %+v, %v; want two questions", view, err)
	}
	var maxScore float64
	labs := 0
	for _, q := range view.Questions {
		maxScore += q.Points
		if q.Pool != assessments.PoolScenario {
			t.Errorf("%s pool = %q; want scenario", q.Code, q.Pool)
		}
		if q.HasLab {
			labs++
		}
	}
	if view.MaxScore != maxScore || view.PassScore != math.Round(maxScore*60)/100 {
		t.Fatalf("attempt max, pass = %v, %v; want %v and 60%% of it", view.MaxScore, view.PassScore, maxScore)
	}
	if got := len(f.flow.labs.questions[attemptID]); got != labs {
		t.Fatalf("labs requested = %d; want one per dealt question with an image (%d)", got, labs)
	}

	// Files are served only for a dealt question, and answers match
	// regardless of case and surrounding space.
	for _, q := range view.Questions {
		if q.ID == logs {
			path, name, err := f.flow.service.File(ctx, userID, attemptID, q.Files[0].ID)
			if err != nil || name != "access.log" || filepath.Dir(path) != f.filesDir {
				t.Fatalf("File() = %q, %q, %v", path, name, err)
			}
		}
	}
	values := map[uint64]map[string]string{
		web:  {"Admin password": " summer2024 ", "Flag": "flag{WEB}"},
		ssh:  {"Flag": "FLAG{SSH}"},
		logs: {"Attacker IP": "10.0.0.7"},
	}
	for _, q := range view.Questions {
		for _, flag := range q.Flags {
			value := values[q.ID][flag.Prompt]
			if result, err := f.flow.service.Submit(ctx, userID, attemptID, flag.ID, value); err != nil || !result.Correct {
				t.Fatalf("Submit(%s) = %+v, %v; want correct", flag.Prompt, result, err)
			}
		}
	}
	summary, err := f.flow.service.Finish(ctx, userID, attemptID)
	if err != nil || summary.Result != "passed" || summary.Score != maxScore {
		t.Fatalf("Finish() = %+v, %v; want passed with %v", summary, err, maxScore)
	}

	// Once sat, the level can neither be unpublished nor deleted.
	if err := f.levels.Unpublish(ctx, f.flow.admin, levelID); !errors.Is(err, levels.ErrInUse) {
		t.Fatalf("Unpublish() after an attempt = %v; want ErrInUse", err)
	}
}

func TestEditingAnswersKeepsOrReplacesDigests(t *testing.T) {
	f := newBuilderFixture(t, 0)
	ctx := context.Background()
	levelID := f.level(t, 1, 50)
	questionID := f.question(t, levelID, "", answer("Port", "8080", 10), answer("Service", "nginx", 10))

	level, _ := f.levels.Get(ctx, levelID)
	port, service := level.Questions[0].Answers[0], level.Questions[0].Answers[1]
	var before []byte
	if err := f.flow.db.QueryRow(`SELECT expected_hmac FROM answer_validators WHERE question_id = ?`, port.ID).Scan(&before); err != nil {
		t.Fatal(err)
	}

	// Port is kept with a blank answer, service is dropped, user is added.
	if err := f.levels.UpdateQuestion(ctx, f.flow.admin, levelID, questionID, levels.QuestionInput{
		Prompt: "Find the service",
		Answers: []levels.AnswerInput{
			{ID: port.ID, Prompt: "Open port", Marks: 15},
			answer("User", "www-data", 5),
		},
	}); err != nil {
		t.Fatalf("UpdateQuestion() error = %v", err)
	}
	level, _ = f.levels.Get(ctx, levelID)
	q := level.Questions[0]
	if len(q.Answers) != 2 || q.Answers[0].ID != port.ID || q.Answers[0].Prompt != "Open port" ||
		q.Answers[1].Prompt != "User" || q.Marks != 20 {
		t.Fatalf("question after update = %+v", q)
	}
	var after []byte
	_ = f.flow.db.QueryRow(`SELECT expected_hmac FROM answer_validators WHERE question_id = ?`, port.ID).Scan(&after)
	if string(after) != string(before) {
		t.Fatal("a blank answer changed the stored digest; want it kept")
	}
	var gone int
	_ = f.flow.db.QueryRow(`SELECT COUNT(*) FROM assessment_questions WHERE id = ?`, service.ID).Scan(&gone)
	if gone != 0 {
		t.Fatal("an unlisted answer was kept; want it removed")
	}

	// A new answer for an existing slot replaces its digest.
	if err := f.levels.UpdateQuestion(ctx, f.flow.admin, levelID, questionID, levels.QuestionInput{
		Prompt:  "Find the service",
		Answers: []levels.AnswerInput{{ID: port.ID, Prompt: "Open port", Answer: "8443", Marks: 15}},
	}); err != nil {
		t.Fatal(err)
	}
	_ = f.flow.db.QueryRow(`SELECT expected_hmac FROM answer_validators WHERE question_id = ?`, port.ID).Scan(&after)
	if string(after) == string(before) {
		t.Fatal("a new answer kept the old digest")
	}

	// Another question's answer cannot be smuggled in by ID.
	other := f.question(t, levelID, "", answer("X", "y", 1))
	level, _ = f.levels.Get(ctx, levelID)
	foreign := level.Questions[1].Answers[0].ID
	err := f.levels.UpdateQuestion(ctx, f.flow.admin, levelID, questionID, levels.QuestionInput{
		Prompt: "p", Answers: []levels.AnswerInput{{ID: foreign, Prompt: "x", Marks: 1}},
	})
	if !errors.Is(err, levels.ErrInvalidInput) {
		t.Fatalf("UpdateQuestion() with another question's answer = %v; want ErrInvalidInput", err)
	}
	_ = other
}

func TestBuilderFilesAndDeletion(t *testing.T) {
	f := newBuilderFixture(t, 16)
	ctx := context.Background()
	levelID := f.level(t, 1, 50)
	questionID := f.question(t, levelID, "", answer("Flag", "x", 10))

	if _, err := f.levels.UploadFile(ctx, f.flow.admin, levelID, questionID, "big.pcap",
		strings.NewReader(strings.Repeat("x", 17))); !errors.Is(err, levels.ErrTooLarge) {
		t.Fatalf("UploadFile(17 bytes, limit 16) = %v; want ErrTooLarge", err)
	}
	file, err := f.levels.UploadFile(ctx, f.flow.admin, levelID, questionID, `C:\Users\me\..\notes "v2".txt`,
		strings.NewReader("hello"))
	if err != nil || file.Name != "notes v2.txt" || file.Size != 5 {
		t.Fatalf("UploadFile() = %+v, %v; want notes v2.txt of 5 bytes", file, err)
	}
	entries, _ := os.ReadDir(f.filesDir)
	if len(entries) != 1 || entries[0].Name() != file.StoredName {
		t.Fatalf("files on disk = %v; want only the stored upload", entries)
	}
	if _, err := f.levels.UploadFile(ctx, f.flow.admin, levelID, questionID+999, "x.txt",
		strings.NewReader("x")); !errors.Is(err, levels.ErrNotFound) {
		t.Fatalf("UploadFile(unknown question) = %v; want ErrNotFound", err)
	}
	if entries, _ := os.ReadDir(f.filesDir); len(entries) != 1 {
		t.Fatalf("a refused upload left %d files on disk; want 1", len(entries))
	}

	if err := f.levels.Delete(ctx, f.flow.admin, levelID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if entries, _ := os.ReadDir(f.filesDir); len(entries) != 0 {
		t.Fatalf("files left after deleting the level: %v", entries)
	}
	if _, err := f.levels.Get(ctx, levelID); !errors.Is(err, levels.ErrNotFound) {
		t.Fatalf("Get() after Delete() = %v; want ErrNotFound", err)
	}
}

func TestBuilderRefusesImportedLevelAndBadInput(t *testing.T) {
	flow := newFlowFixture(t) // Level 1, imported
	builder, err := levels.NewService(NewLevelRepository(flow.db), NewAssessmentRepository(flow.db), zap.NewNop(),
		levels.Config{AnswerKey: assessmentKey, FilesDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	list, err := builder.List(ctx)
	if err != nil || len(list) != 1 || list[0].Built || list[0].PassPercent != 80 || list[0].QuestionCount != 3 {
		t.Fatalf("List() = %+v, %v; want Level 1, imported, 80%%", list, err)
	}
	if err := builder.Update(ctx, flow.admin, list[0].ID, levels.LevelInput{
		Name: "x", DurationMinutes: 1, QuestionsPerAttempt: 1, PassPercent: 1,
	}); !errors.Is(err, levels.ErrReadOnly) {
		t.Fatalf("Update(Level 1) = %v; want ErrReadOnly", err)
	}

	levelID, err := builder.Create(ctx, flow.admin, levels.LevelInput{
		Name: "Level 2", DurationMinutes: 60, QuestionsPerAttempt: 3, PassPercent: 70,
	})
	if err != nil {
		t.Fatal(err)
	}
	bad := []levels.QuestionInput{
		{Prompt: "p", Image: "Scenario2", Answers: []levels.AnswerInput{answer("a", "b", 1)}},
		{Prompt: "p", Image: "web\nimage: evil", Answers: []levels.AnswerInput{answer("a", "b", 1)}},
		{Prompt: "p", Answers: []levels.AnswerInput{{Prompt: "a", Marks: 1}}},
		{Prompt: "p", Answers: []levels.AnswerInput{answer("a", "b", 0)}},
		{Prompt: "", Answers: []levels.AnswerInput{answer("a", "b", 1)}},
		{Prompt: "p"},
	}
	for i, in := range bad {
		if _, err := builder.CreateQuestion(ctx, flow.admin, levelID, in); !errors.Is(err, levels.ErrInvalidInput) {
			t.Errorf("CreateQuestion(bad %d) = %v; want ErrInvalidInput", i, err)
		}
	}

	// The new level comes after Level 1 and is not in the catalogue yet.
	level, _ := builder.Get(ctx, levelID)
	if level.Sequence != 2 {
		t.Fatalf("new level sequence = %d; want 2", level.Sequence)
	}
	bank, _ := flow.service.QuestionBank(ctx)
	if len(bank) != 1 {
		t.Fatalf("catalogue has %d assessments; want only Level 1 while Level 2 is a draft", len(bank))
	}
}
