package levels

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/assessments"
	"github.com/G6kco/CyberSpace/internal/evaluation"
	"github.com/G6kco/CyberSpace/internal/labs"
)

// Limits on what an administrator may enter.
const (
	maxNameLength         = 120
	maxTitleLength        = 200
	maxInstructionsLength = 20000
	maxPromptLength       = 20000
	maxAnswerPromptLength = 1000
	// maxAnswerLength matches the longest answer a student may submit.
	maxAnswerLength     = 256
	maxAnswers          = 20
	maxDurationMinutes  = 600
	maxQuestionsDealt   = 50
	maxMarks            = 1000
	maxImageLength      = 160 // lab_templates.slug
	maxFileNameLength   = 255
	defaultMaxFileBytes = 100 << 20
)

// imagePattern admits Docker image references ("scenario2",
// "registry:5000/team/web-login:v1", "image@sha256:…") and nothing that
// could change the meaning of the YAML spec the image is written into.
var imagePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]*$`)

// Config holds the builder's settings.
type Config struct {
	// AnswerKey keys the stored answer digests; see evaluation.Digest.
	AnswerKey []byte
	// FilesDir holds uploaded question files.
	FilesDir string
	// MaxFileBytes bounds one upload; zero means 100 MiB.
	MaxFileBytes int64
}

// Service applies the level builder's rules over storage.
type Service struct {
	repo    Repository
	auditor Auditor
	logger  *zap.Logger
	cfg     Config
	now     func() time.Time
}

func NewService(repo Repository, auditor Auditor, logger *zap.Logger, cfg Config) (*Service, error) {
	if len(cfg.AnswerKey) < evaluation.MinKeyLength {
		return nil, fmt.Errorf("levels: %w", evaluation.ErrShortKey)
	}
	if cfg.FilesDir == "" {
		return nil, errors.New("levels: a files directory is required")
	}
	if cfg.MaxFileBytes <= 0 {
		cfg.MaxFileBytes = defaultMaxFileBytes
	}
	return &Service{
		repo:    repo,
		auditor: auditor,
		logger:  logger,
		cfg:     cfg,
		now:     func() time.Time { return time.Now().UTC() },
	}, nil
}

// MaxFileBytes is the largest file one upload may carry.
func (s *Service) MaxFileBytes() int64 { return s.cfg.MaxFileBytes }

// ---- Levels ----------------------------------------------------------------

func (s *Service) List(ctx context.Context) ([]Level, error) {
	return s.repo.List(ctx)
}

// Get returns a level with its questions, reporting whether each file is
// present on disk.
func (s *Service) Get(ctx context.Context, levelID uint64) (Level, error) {
	level, err := s.repo.Get(ctx, levelID)
	if err != nil {
		return Level{}, err
	}
	for i := range level.Questions {
		for j := range level.Questions[i].Files {
			f := &level.Questions[i].Files[j]
			_, statErr := os.Stat(s.filePath(f.StoredName))
			f.Present = statErr == nil
		}
	}
	return level, nil
}

func (s *Service) Create(ctx context.Context, actor assessments.Actor, in LevelInput) (uint64, error) {
	in, err := cleanLevel(in)
	var id uint64
	if err == nil {
		id, err = s.repo.Create(ctx, actor.UserID, in)
	}
	s.audit(ctx, actor, "level.create", id, in, err)
	return id, err
}

func (s *Service) Update(ctx context.Context, actor assessments.Actor, levelID uint64, in LevelInput) error {
	in, err := cleanLevel(in)
	if err == nil {
		err = s.repo.Update(ctx, levelID, in)
	}
	s.audit(ctx, actor, "level.update", levelID, in, err)
	return err
}

func (s *Service) Delete(ctx context.Context, actor assessments.Actor, levelID uint64) error {
	stored, err := s.repo.Delete(ctx, levelID)
	if err == nil {
		s.removeFiles(stored...)
	}
	s.audit(ctx, actor, "level.delete", levelID, nil, err)
	return err
}

func (s *Service) Publish(ctx context.Context, actor assessments.Actor, levelID uint64) error {
	err := s.repo.Publish(ctx, levelID, s.now())
	s.audit(ctx, actor, "level.publish", levelID, nil, err)
	return err
}

func (s *Service) Unpublish(ctx context.Context, actor assessments.Actor, levelID uint64) error {
	err := s.repo.Unpublish(ctx, actor.UserID, levelID, s.now())
	s.audit(ctx, actor, "level.unpublish", levelID, nil, err)
	return err
}

// ---- Questions -------------------------------------------------------------

func (s *Service) CreateQuestion(ctx context.Context, actor assessments.Actor, levelID uint64, in QuestionInput) (uint64, error) {
	in, err := s.cleanQuestion(in)
	var id uint64
	if err == nil {
		id, err = s.repo.CreateQuestion(ctx, actor.UserID, levelID, in)
	}
	s.audit(ctx, actor, "question.create", levelID, auditQuestion(id, in), err)
	return id, err
}

func (s *Service) UpdateQuestion(ctx context.Context, actor assessments.Actor, levelID, questionID uint64, in QuestionInput) error {
	in, err := s.cleanQuestion(in)
	if err == nil {
		err = s.repo.UpdateQuestion(ctx, actor.UserID, levelID, questionID, in)
	}
	s.audit(ctx, actor, "question.update", levelID, auditQuestion(questionID, in), err)
	return err
}

func (s *Service) DeleteQuestion(ctx context.Context, actor assessments.Actor, levelID, questionID uint64) error {
	stored, err := s.repo.DeleteQuestion(ctx, levelID, questionID)
	if err == nil {
		s.removeFiles(stored...)
	}
	s.audit(ctx, actor, "question.delete", levelID, map[string]uint64{"questionId": questionID}, err)
	return err
}

// ---- Files -----------------------------------------------------------------

// UploadFile stores a file for a question under a random name, then records
// it. The original name is kept only for the download.
func (s *Service) UploadFile(
	ctx context.Context,
	actor assessments.Actor,
	levelID, questionID uint64,
	name string,
	body io.Reader,
) (File, error) {
	file, err := s.uploadFile(ctx, actor, levelID, questionID, name, body)
	s.audit(ctx, actor, "file.upload", levelID, map[string]any{
		"questionId": questionID, "fileId": file.ID, "name": file.Name, "size": file.Size,
	}, err)
	return file, err
}

func (s *Service) uploadFile(
	ctx context.Context,
	actor assessments.Actor,
	levelID, questionID uint64,
	name string,
	body io.Reader,
) (File, error) {
	name = cleanFileName(name)
	if name == "" {
		return File{}, fmt.Errorf("%w: the file needs a name", ErrInvalidInput)
	}
	stored, err := randomName()
	if err != nil {
		return File{}, err
	}
	if err := os.MkdirAll(s.cfg.FilesDir, 0o750); err != nil {
		return File{}, err
	}

	// Written to a temporary name first, so a half-written upload is never
	// served under a recorded name.
	temp, err := os.CreateTemp(s.cfg.FilesDir, ".upload-*")
	if err != nil {
		return File{}, err
	}
	defer os.Remove(temp.Name()) // no-op once renamed

	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(body, s.cfg.MaxFileBytes+1))
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return File{}, err
	}
	if size > s.cfg.MaxFileBytes {
		return File{}, ErrTooLarge
	}
	if size == 0 {
		return File{}, fmt.Errorf("%w: the file is empty", ErrInvalidInput)
	}

	final := s.filePath(stored)
	if err := os.Rename(temp.Name(), final); err != nil {
		return File{}, err
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))

	id, err := s.repo.AddFile(ctx, actor.UserID, levelID, NewFile{
		QuestionID:   questionID,
		StoredName:   stored,
		OriginalName: name,
		Size:         size,
		SHA256:       digest,
	})
	if err != nil {
		s.removeFiles(stored)
		return File{}, err
	}
	return File{ID: id, Name: name, Size: size, Present: true, StoredName: stored}, nil
}

func (s *Service) DeleteFile(ctx context.Context, actor assessments.Actor, levelID, questionID, fileID uint64) error {
	stored, err := s.repo.DeleteFile(ctx, levelID, questionID, fileID)
	if err == nil {
		s.removeFiles(stored)
	}
	s.audit(ctx, actor, "file.delete", levelID, map[string]uint64{"questionId": questionID, "fileId": fileID}, err)
	return err
}

// FilePath returns where a question's file is stored and its download name.
func (s *Service) FilePath(ctx context.Context, levelID, questionID, fileID uint64) (string, string, error) {
	f, err := s.repo.File(ctx, levelID, questionID, fileID)
	if err != nil {
		return "", "", err
	}
	p := s.filePath(f.StoredName)
	if _, err := os.Stat(p); err != nil {
		return "", "", fmt.Errorf("%w: the file is missing from the server", ErrNotFound)
	}
	return p, f.Name, nil
}

// ---- Helpers ---------------------------------------------------------------

// filePath joins a stored name onto the files directory. Base is a guard:
// stored names are 32 hex digits by CHECK constraint.
func (s *Service) filePath(stored string) string {
	return filepath.Join(s.cfg.FilesDir, filepath.Base(stored))
}

func (s *Service) removeFiles(stored ...string) {
	for _, name := range stored {
		if err := os.Remove(s.filePath(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Error("Removing question file failed", zap.String("file", name), zap.Error(err))
		}
	}
}

func (s *Service) audit(ctx context.Context, actor assessments.Actor, action string, levelID uint64, after any, err error) {
	outcome := "success"
	if err != nil {
		outcome = "failure"
		after = map[string]string{"error": err.Error()}
	}
	entityID := ""
	if levelID != 0 {
		entityID = strconv.FormatUint(levelID, 10)
	}
	if auditErr := s.auditor.Audit(ctx, assessments.AuditEntry{
		Actor:      actor,
		Action:     action,
		Outcome:    outcome,
		EntityType: "level",
		EntityID:   entityID,
		After:      after,
	}); auditErr != nil {
		s.logger.Error("Audit write failed", zap.String("action", action), zap.Error(auditErr))
	}
}

// auditQuestion is what the audit log keeps of a question: never answers.
func auditQuestion(id uint64, in QuestionInput) map[string]any {
	marks := 0.0
	for _, a := range in.Answers {
		marks += a.Marks
	}
	return map[string]any{"questionId": id, "image": in.Image, "answers": len(in.Answers), "marks": marks}
}

func cleanLevel(in LevelInput) (LevelInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Title = strings.TrimSpace(in.Title)
	in.Instructions = strings.TrimSpace(in.Instructions)
	if in.Title == "" {
		in.Title = in.Name
	}
	switch {
	case in.Name == "" || utf8.RuneCountInString(in.Name) > maxNameLength:
		return in, invalid("the name must be 1 to %d characters", maxNameLength)
	case utf8.RuneCountInString(in.Title) > maxTitleLength:
		return in, invalid("the title must be at most %d characters", maxTitleLength)
	case utf8.RuneCountInString(in.Instructions) > maxInstructionsLength:
		return in, invalid("the instructions must be at most %d characters", maxInstructionsLength)
	case in.DurationMinutes < 1 || in.DurationMinutes > maxDurationMinutes:
		return in, invalid("the duration must be 1 to %d minutes", maxDurationMinutes)
	case in.QuestionsPerAttempt < 1 || in.QuestionsPerAttempt > maxQuestionsDealt:
		return in, invalid("questions per student must be 1 to %d", maxQuestionsDealt)
	}
	in.PassPercent = math.Round(in.PassPercent*100) / 100
	if in.PassPercent <= 0 || in.PassPercent > 100 {
		return in, invalid("the pass mark must be above 0%% and at most 100%%")
	}
	return in, nil
}

func (s *Service) cleanQuestion(in QuestionInput) (QuestionInput, error) {
	in.Prompt = strings.TrimSpace(in.Prompt)
	in.Image = strings.TrimSpace(in.Image)
	switch {
	case in.Prompt == "" || utf8.RuneCountInString(in.Prompt) > maxPromptLength:
		return in, invalid("the question must be 1 to %d characters", maxPromptLength)
	case len(in.Answers) == 0 || len(in.Answers) > maxAnswers:
		return in, invalid("a question needs 1 to %d answers", maxAnswers)
	}
	if in.Image != "" {
		if len(in.Image) > maxImageLength || !imagePattern.MatchString(in.Image) {
			return in, invalid("the image must be a Docker image name such as scenario2 or registry:5000/web:v1")
		}
		if _, err := labs.ParseSpec("image: " + in.Image + "\n"); err != nil {
			return in, invalid("%s", err.Error())
		}
	}

	seen := map[uint64]bool{}
	for i := range in.Answers {
		a := &in.Answers[i]
		a.Prompt = strings.TrimSpace(a.Prompt)
		a.Answer = strings.TrimSpace(a.Answer)
		a.Marks = math.Round(a.Marks*100) / 100
		n := i + 1
		switch {
		case a.Prompt == "" || utf8.RuneCountInString(a.Prompt) > maxAnswerPromptLength:
			return in, invalid("answer %d needs a prompt of 1 to %d characters", n, maxAnswerPromptLength)
		case a.Marks <= 0 || a.Marks > maxMarks:
			return in, invalid("answer %d must be worth more than 0 and at most %d marks", n, maxMarks)
		case utf8.RuneCountInString(a.Answer) > maxAnswerLength:
			return in, invalid("answer %d must be at most %d characters", n, maxAnswerLength)
		case a.Answer == "" && a.ID == 0:
			return in, invalid("answer %d needs its expected answer", n)
		case a.ID != 0 && seen[a.ID]:
			return in, invalid("answer %d is listed twice", n)
		}
		seen[a.ID] = a.ID != 0
		a.Digest = nil
		if a.Answer != "" {
			d := evaluation.Digest(s.cfg.AnswerKey, a.Answer)
			a.Digest = d[:]
		}
		// The expected answer goes no further than its digest.
		a.Answer = ""
	}
	return in, nil
}

// cleanFileName keeps the last path element of an uploaded name, without
// control characters, bounded in length.
func cleanFileName(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "." || name == "/" || name == ".." {
		return ""
	}
	if r := []rune(name); len(r) > maxFileNameLength {
		ext := []rune(filepath.Ext(name))
		if len(ext) > 16 {
			ext = nil
		}
		name = string(r[:maxFileNameLength-len(ext)]) + string(ext)
	}
	return name
}

func randomName() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidInput}, args...)...)
}

// Message is the user-facing text for a builder error.
func Message(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "Not found."
	case errors.Is(err, ErrPublished):
		return "The level is published. Unpublish it before changing it."
	case errors.Is(err, ErrReadOnly):
		return "This level was imported and cannot be edited here."
	case errors.Is(err, ErrInUse):
		return "Students have already taken or been admitted to this level, so it can no longer be changed this way."
	case errors.Is(err, ErrTooLarge):
		return "The file is too large."
	case errors.Is(err, ErrNotReady), errors.Is(err, ErrInvalidInput):
		msg := err.Error()
		for _, prefix := range []string{ErrNotReady.Error() + ": ", ErrInvalidInput.Error() + ": "} {
			msg = strings.TrimPrefix(msg, prefix)
		}
		return msg
	default:
		return "The action failed."
	}
}
