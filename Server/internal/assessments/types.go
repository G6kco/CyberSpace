// Package assessments runs level assessments: a student enters the test
// portal, an administrator takes attendance and starts the test, each
// student is dealt one question per pool and gets a lab, submits flags, and
// the attempt is scored when submitted, ended, or out of time.
package assessments

import (
	"context"
	"errors"
	"net"
	"time"
)

var (
	// ErrNotFound covers attempts and questions the caller may not see, so a
	// student cannot probe other students' attempt IDs.
	ErrNotFound = errors.New("not found")
	// ErrNotEligible means the student cannot take an assessment now.
	ErrNotEligible = errors.New("student is not eligible for an assessment now")
	// ErrNotInLobby means attendance was taken for a student not in the portal.
	ErrNotInLobby = errors.New("student is not in the test portal")
	// ErrNoAttendance means a test was started for a student without attendance.
	ErrNoAttendance = errors.New("attendance has not been taken for this student")
	// ErrAttemptClosed means the attempt is no longer in progress.
	ErrAttemptClosed = errors.New("attempt is not in progress")
	// ErrAlreadySolved means the flag was already answered correctly.
	ErrAlreadySolved = errors.New("flag already solved")
	// ErrTooManySubmissions means the student is submitting too fast or has
	// used every allowed submission for the flag.
	ErrTooManySubmissions = errors.New("too many submissions")
	// ErrInvalidInput means the request itself is malformed.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNoLab means the attempt has no Nmap question to provision a lab for.
	ErrNoLab = errors.New("attempt has no lab")
	// ErrLabActive means the attempt's lab is still running or starting.
	ErrLabActive = errors.New("lab is still active")
)

// Student statuses reported by Overview.
const (
	StatusEligible    = "eligible"
	StatusCooldown    = "cooldown"
	StatusInProgress  = "in_progress"
	StatusCompleted   = "completed"
	StatusUnavailable = "unavailable"
)

// Pools a level deals one question from each.
var Pools = []string{"wireshark", "nmap"}

// Assessment is the published assessment of one level.
type Assessment struct {
	LevelID         uint64  `json:"-"`
	LevelName       string  `json:"level"`
	LevelSequence   int     `json:"levelNumber"`
	AssessmentID    uint64  `json:"-"`
	Title           string  `json:"title"`
	VersionID       uint64  `json:"-"`
	DurationSeconds int     `json:"durationSeconds"`
	PassScore       float64 `json:"passScore"`
	TotalPoints     float64 `json:"totalPoints"`
	Instructions    string  `json:"instructions"`
}

// StudentState is what storage knows about a student's assessment record.
type StudentState struct {
	PassedLevels map[uint64]bool
	// LastFailure maps a level to when its latest unpassed attempt ended.
	LastFailure     map[uint64]time.Time
	ActiveAttemptID uint64
	// AttendanceVersions are versions with an approved, unconsumed grant
	// whose window covers now.
	AttendanceVersions map[uint64]bool
	History            []AttemptSummary
}

// AttemptSummary is one attempt in a history or listing.
type AttemptSummary struct {
	ID         uint64     `json:"id"`
	Level      string     `json:"level"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	Result     string     `json:"result"`
	Score      float64    `json:"score"`
	MaxScore   float64    `json:"maxScore"`
	PassScore  float64    `json:"passScore"`
	StartedAt  *time.Time `json:"startedAt"`
	DeadlineAt *time.Time `json:"deadlineAt"`
	EndedAt    *time.Time `json:"endedAt"`
	Reason     string     `json:"reason,omitempty"`
}

// Overview is the student's assessment page.
type Overview struct {
	Status           string           `json:"status"`
	Assessment       *Assessment      `json:"assessment"`
	CooldownUntil    *time.Time       `json:"cooldownUntil"`
	AttendanceMarked bool             `json:"attendanceMarked"`
	InLobby          bool             `json:"inLobby"`
	ActiveAttemptID  *uint64          `json:"activeAttemptId"`
	History          []AttemptSummary `json:"history"`
	ServerTime       time.Time        `json:"serverTime"`
}

// AttemptView is an attempt as its student sees it. Questions are present
// only while the attempt is in progress, so the bank is not revealed after.
type AttemptView struct {
	AttemptSummary
	Questions  []DealtQuestion `json:"questions"`
	Lab        *LabView        `json:"lab"`
	ServerTime time.Time       `json:"serverTime"`
}

// DealtQuestion is one question dealt to an attempt.
type DealtQuestion struct {
	ID         uint64     `json:"id"`
	Code       string     `json:"code"`
	Pool       string     `json:"pool"`
	Difficulty string     `json:"difficulty"`
	Prompt     string     `json:"prompt"`
	Points     float64    `json:"points"`
	Score      float64    `json:"score"`
	PcapFile   string     `json:"-"`
	HasPcap    bool       `json:"hasPcap"`
	Flags      []FlagView `json:"flags"`
	// LabTemplateVersionID is the Nmap image's lab, or zero.
	LabTemplateVersionID uint64 `json:"-"`
}

// FlagView is one answer slot; it never carries the expected answer.
type FlagView struct {
	ID          uint64  `json:"id"`
	Prompt      string  `json:"prompt"`
	Points      float64 `json:"points"`
	Solved      bool    `json:"solved"`
	Submissions int     `json:"submissions"`
}

// LabView is the state of an attempt's lab. IP is set only while it runs.
type LabView struct {
	Status string `json:"status"`
	IP     string `json:"ip,omitempty"`
}

// SubmitResult reports a flag submission.
type SubmitResult struct {
	Correct bool    `json:"correct"`
	Points  float64 `json:"points"`
	Score   float64 `json:"score"`
}

// Finish modes.
const (
	FinishSubmit = "submit" // the student submitted
	FinishEnd    = "end"    // an administrator ended it; it is still scored
	FinishExpire = "expire" // the deadline passed; it is scored
	FinishRevoke = "revoke" // an administrator revoked it; it is not scored
)

// StartRequest starts one student's attempt.
type StartRequest struct {
	AdminID         uint64
	UserID          uint64
	VersionID       uint64
	DurationSeconds int
	MaxScore        float64
	Now             time.Time
}

// Started is a committed attempt and the lab it needs, if any.
type Started struct {
	AttemptID            uint64
	LabTemplateVersionID uint64
}

// Student identifies a student for administrator views.
type Student struct {
	ID             uint64 `json:"-"`
	PublicID       string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	RegisterNumber string `json:"registerNumber"`
	Status         string `json:"status"`
}

// LobbyEntry is a student in the test portal or with attendance taken.
type LobbyEntry struct {
	Student
	Level            string     `json:"level"`
	Online           bool       `json:"online"`
	LastSeen         *time.Time `json:"lastSeen"`
	AttendanceMarked bool       `json:"attendanceMarked"`
	ActiveAttemptID  *uint64    `json:"activeAttemptId"`
	Eligibility      string     `json:"eligibility"`
}

// AdminAttempt is one attempt in the administrator's monitoring view.
type AdminAttempt struct {
	AttemptSummary
	Student   Student         `json:"student"`
	Lab       *LabView        `json:"lab"`
	Questions []DealtQuestion `json:"questions"`
}

// BankAssessment is one level's assessment with its question bank.
type BankAssessment struct {
	Assessment
	Questions []BankQuestion `json:"questions"`
}

// BankQuestion describes a bank question to administrators, without answers.
type BankQuestion struct {
	ID          uint64   `json:"id"`
	Code        string   `json:"code"`
	Pool        string   `json:"pool"`
	Difficulty  string   `json:"difficulty"`
	Prompt      string   `json:"prompt"`
	Points      float64  `json:"points"`
	FlagPrompts []string `json:"flagPrompts"`
	PcapFile    string   `json:"pcapFile,omitempty"`
	PcapPresent bool     `json:"pcapPresent"`
	Image       string   `json:"image,omitempty"`
	TimesDealt  int      `json:"timesDealt"`
}

// StudentRecord is one row of the administrator's student list.
type StudentRecord struct {
	Student
	LevelsPassed  int             `json:"levelsPassed"`
	LastAttempt   *AttemptSummary `json:"lastAttempt"`
	CooldownUntil *time.Time      `json:"cooldownUntil"`
}

// Actor is who performed an administrative action, for the audit log.
type Actor struct {
	UserID    uint64
	IP        net.IP
	UserAgent string
}

// AuditEntry is one audit_logs row.
type AuditEntry struct {
	Actor      Actor
	Action     string
	Outcome    string
	EntityType string
	EntityID   string
	After      any
}

// Repository is the storage the service needs.
type Repository interface {
	Catalog(ctx context.Context) ([]Assessment, error)
	StudentState(ctx context.Context, userID uint64, now time.Time) (StudentState, error)
	Students(ctx context.Context, ids []uint64) ([]Student, error)
	StudentByPublicID(ctx context.Context, publicID string) (Student, error)
	StudentRecords(ctx context.Context, search string, limit int) ([]StudentRecord, error)

	MarkAttendance(ctx context.Context, adminID, userID, versionID uint64, now, until time.Time) error
	CancelAttendance(ctx context.Context, adminID, userID uint64, now time.Time, reason string) (int64, error)
	AttendanceUsers(ctx context.Context, now time.Time) (map[uint64]uint64, error)

	StartAttempt(ctx context.Context, req StartRequest) (Started, error)
	Attempt(ctx context.Context, attemptID uint64) (AttemptView, uint64, error)
	SubmitAnswer(ctx context.Context, attemptID, userID, flagID uint64, digest [32]byte, now time.Time) (SubmitResult, error)
	FinishAttempt(ctx context.Context, attemptID, actorID uint64, mode, reason string, now time.Time) (AttemptSummary, error)
	DueAttempts(ctx context.Context, now time.Time) ([]uint64, error)
	AttemptLab(ctx context.Context, attemptID uint64) (uint64, error)

	ListAttempts(ctx context.Context, active bool, limit int) ([]AdminAttempt, error)
	QuestionBank(ctx context.Context) ([]BankAssessment, error)

	Audit(ctx context.Context, entry AuditEntry) error
}

// Labs is the lab engine's request side.
type Labs interface {
	RequestProvision(ctx context.Context, attemptID, templateVersionID uint64) (uint64, error)
	RequestStop(ctx context.Context, attemptID uint64) error
}
