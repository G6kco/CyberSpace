// Package levels lets administrators build assessment levels: a level is a
// bank of scenario questions, each with its answers and their marks, an
// optional lab image, and optional files to download. A student sitting the
// level is dealt QuestionsPerAttempt random questions and passes at
// PassPercent of the marks of the questions dealt to them.
//
// A level is edited only while it is a draft. Publishing it adds it to the
// student catalogue after the levels before it; unpublishing is allowed
// until the first attempt, after which the level is fixed.
package levels

import (
	"context"
	"errors"
	"time"

	"github.com/G6kco/CyberSpace/internal/assessments"
)

var (
	// ErrNotFound covers levels, questions, answers and files that do not
	// exist or do not belong together.
	ErrNotFound = errors.New("not found")
	// ErrInvalidInput means the request itself is malformed.
	ErrInvalidInput = errors.New("invalid input")
	// ErrPublished means a published level was asked to change.
	ErrPublished = errors.New("level is published")
	// ErrReadOnly means the level was imported (Level 1), not built here.
	ErrReadOnly = errors.New("level was not built in the level builder")
	// ErrInUse means students have sat the level or had attendance taken
	// for it, so it can no longer be unpublished or deleted.
	ErrInUse = errors.New("level is in use")
	// ErrNotReady means the level cannot be published yet.
	ErrNotReady = errors.New("level is not ready to publish")
	// ErrTooLarge means an uploaded file exceeds the size limit.
	ErrTooLarge = errors.New("file too large")
)

// Level statuses.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Level is one level as administrators see it. Expected answers never
// appear: they are stored only as keyed digests.
type Level struct {
	ID                  uint64  `json:"id"`
	Sequence            int     `json:"sequence"`
	Name                string  `json:"name"`
	Title               string  `json:"title"`
	Instructions        string  `json:"instructions"`
	DurationMinutes     int     `json:"durationMinutes"`
	QuestionsPerAttempt int     `json:"questionsPerAttempt"`
	PassPercent         float64 `json:"passPercent"`
	Status              string  `json:"status"`
	// Built is false for Level 1, which was imported and deals one question
	// per pool; it is shown but cannot be edited here.
	Built         bool       `json:"built"`
	QuestionCount int        `json:"questionCount"`
	Attempts      int        `json:"attempts"`
	Questions     []Question `json:"questions,omitempty"`
}

// Question is one scenario of a level.
type Question struct {
	ID      uint64   `json:"id"`
	Code    string   `json:"code"`
	Prompt  string   `json:"prompt"`
	Image   string   `json:"image"`
	Marks   float64  `json:"marks"`
	Answers []Answer `json:"answers"`
	Files   []File   `json:"files"`
}

// Answer is one answer slot of a question, without its expected value.
type Answer struct {
	ID     uint64  `json:"id"`
	Prompt string  `json:"prompt"`
	Marks  float64 `json:"marks"`
}

// File is a file students download for a question.
type File struct {
	ID         uint64 `json:"id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Present    bool   `json:"present"`
	StoredName string `json:"-"`
}

// LevelInput creates or updates a level's settings.
type LevelInput struct {
	Name                string  `json:"name"`
	Title               string  `json:"title"`
	Instructions        string  `json:"instructions"`
	DurationMinutes     int     `json:"durationMinutes"`
	QuestionsPerAttempt int     `json:"questionsPerAttempt"`
	PassPercent         float64 `json:"passPercent"`
}

// QuestionInput creates or replaces a question. Image is optional; an empty
// image means the question has no lab.
type QuestionInput struct {
	Prompt  string        `json:"prompt"`
	Image   string        `json:"image"`
	Answers []AnswerInput `json:"answers"`
}

// AnswerInput is one answer slot. ID identifies an existing slot when
// editing; Answer may then be left empty to keep the stored answer.
type AnswerInput struct {
	ID     uint64  `json:"id"`
	Prompt string  `json:"prompt"`
	Answer string  `json:"answer"`
	Marks  float64 `json:"marks"`
	// Digest is the keyed digest of Answer, set by the service; nil keeps
	// the stored digest of an existing slot.
	Digest []byte `json:"-"`
}

// NewFile is an uploaded file already written to the files directory.
type NewFile struct {
	QuestionID   uint64
	StoredName   string
	OriginalName string
	Size         int64
	SHA256       [32]byte
}

// Repository is the storage the level builder needs. Every change runs in
// one transaction that locks the level and refuses unless it is a built
// draft.
type Repository interface {
	List(ctx context.Context) ([]Level, error)
	Get(ctx context.Context, levelID uint64) (Level, error)

	Create(ctx context.Context, adminID uint64, in LevelInput) (uint64, error)
	Update(ctx context.Context, levelID uint64, in LevelInput) error
	// Delete removes a draft level and returns its files' stored names.
	Delete(ctx context.Context, levelID uint64) ([]string, error)
	Publish(ctx context.Context, levelID uint64, now time.Time) error
	Unpublish(ctx context.Context, adminID, levelID uint64, now time.Time) error

	CreateQuestion(ctx context.Context, adminID, levelID uint64, in QuestionInput) (uint64, error)
	UpdateQuestion(ctx context.Context, adminID, levelID, questionID uint64, in QuestionInput) error
	// DeleteQuestion returns the stored names of the question's files.
	DeleteQuestion(ctx context.Context, levelID, questionID uint64) ([]string, error)

	AddFile(ctx context.Context, adminID, levelID uint64, f NewFile) (uint64, error)
	// DeleteFile returns the file's stored name.
	DeleteFile(ctx context.Context, levelID, questionID, fileID uint64) (string, error)
	File(ctx context.Context, levelID, questionID, fileID uint64) (File, error)
}

// Auditor records administrator actions.
type Auditor interface {
	Audit(ctx context.Context, entry assessments.AuditEntry) error
}
