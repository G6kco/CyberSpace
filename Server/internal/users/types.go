// Package users creates accounts: students in bulk from an uploaded
// workbook, administrators one at a time. Sign-in never creates an account,
// so these are the only ways in.
package users

import (
	"context"
	"errors"
	"time"

	"github.com/G6kco/CyberSpace/internal/assessments"
)

var (
	// ErrInvalidInput wraps a message for the administrator.
	ErrInvalidInput = errors.New("invalid input")
	// ErrConflict means the account already exists or an import collided
	// with a concurrent change.
	ErrConflict = errors.New("conflict")
	// ErrTooLarge means the upload exceeds the size limit.
	ErrTooLarge = errors.New("file too large")
)

// StudentRow is one validated row of an uploaded student list.
type StudentRow struct {
	Sheet          string
	Row            int
	RegisterNumber string
	Name           string
	Email          string
	Department     string
}

// RowIssue is a row that was not imported, and why.
type RowIssue struct {
	Sheet          string `json:"sheet"`
	Row            int    `json:"row"`
	RegisterNumber string `json:"registerNumber"`
	Email          string `json:"email"`
	Reason         string `json:"reason"`
}

// ImportResult counts what an import did. Skipped rows are listed so the
// administrator can correct the workbook and upload it again; importing the
// same list twice changes nothing.
type ImportResult struct {
	Created   int        `json:"created"`
	Updated   int        `json:"updated"`
	Unchanged int        `json:"unchanged"`
	Skipped   []RowIssue `json:"skipped"`
}

// AdminInput is a new administrator.
type AdminInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Admin is an administrator account.
type Admin struct {
	ID          uint64     `json:"id"`
	PublicID    string     `json:"publicId"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// Repository is the storage the service needs.
type Repository interface {
	// ImportStudents creates or updates the students in one transaction,
	// keyed by email. Rows that would take an administrator's email or
	// another student's register number are returned as issues.
	ImportStudents(ctx context.Context, rows []StudentRow) (ImportResult, error)
	// CreateAdmin returns ErrConflict when the email has an account.
	CreateAdmin(ctx context.Context, in AdminInput) (Admin, error)
	Admins(ctx context.Context) ([]Admin, error)
}

// Auditor records administrator actions.
type Auditor interface {
	Audit(ctx context.Context, entry assessments.AuditEntry) error
}
