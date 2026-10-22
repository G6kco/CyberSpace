package users

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"

	"github.com/G6kco/CyberSpace/internal/assessments"
)

// Column widths in users and student_profiles.
const (
	maxEmailLength      = 254
	maxNameLength       = 150
	maxRegisterLength   = 64
	maxDepartmentLength = 120
	// DefaultMaxUploadBytes bounds one uploaded workbook.
	DefaultMaxUploadBytes = 20 << 20
)

// registerPattern admits register numbers as the college writes them
// ("7376222AG102", "201ME153", "2025UAG1024").
var registerPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9/._-]*$`)

// Config holds the service's settings.
type Config struct {
	// AllowedDomain is the Google Workspace domain sign-in admits. An account
	// on any other domain could never sign in, so it is refused here.
	AllowedDomain string
	// MaxUploadBytes bounds one workbook; zero means 20 MiB.
	MaxUploadBytes int64
}

// Service applies the account rules over storage.
type Service struct {
	repo    Repository
	auditor Auditor
	logger  *zap.Logger
	cfg     Config
}

func NewService(repo Repository, auditor Auditor, logger *zap.Logger, cfg Config) (*Service, error) {
	cfg.AllowedDomain = strings.ToLower(strings.TrimSpace(cfg.AllowedDomain))
	if cfg.AllowedDomain == "" {
		return nil, errors.New("users: an allowed email domain is required")
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = DefaultMaxUploadBytes
	}
	return &Service{repo: repo, auditor: auditor, logger: logger, cfg: cfg}, nil
}

// MaxUploadBytes is the largest workbook one upload may carry.
func (s *Service) MaxUploadBytes() int64 { return s.cfg.MaxUploadBytes }

// ImportStudents reads an .xlsx student list and creates or updates a
// student for every valid row. Invalid or conflicting rows are skipped and
// reported; the rest are applied in one transaction.
func (s *Service) ImportStudents(ctx context.Context, actor assessments.Actor, r io.Reader) (ImportResult, error) {
	result, err := s.importStudents(ctx, r)
	after := any(map[string]int{
		"created": result.Created, "updated": result.Updated,
		"unchanged": result.Unchanged, "skipped": len(result.Skipped),
	})
	s.audit(ctx, actor, "students.import", "", after, err)
	return result, err
}

func (s *Service) importStudents(ctx context.Context, r io.Reader) (ImportResult, error) {
	raw, issues, err := readWorkbook(r)
	if err != nil {
		return ImportResult{}, err
	}
	rows, rowIssues := s.cleanRows(raw)
	issues = append(issues, rowIssues...)

	result := ImportResult{Skipped: []RowIssue{}}
	if len(rows) > 0 {
		result, err = s.repo.ImportStudents(ctx, rows)
		if err != nil {
			return ImportResult{}, err
		}
	}
	// Never nil, so the response always carries a list.
	result.Skipped = append(append([]RowIssue{}, issues...), result.Skipped...)
	return result, nil
}

// cleanRows validates each row and drops duplicates. A row repeated exactly
// (the same student on two year sheets) is imported once; two rows sharing
// an email or register number with different details are both skipped,
// because which one is right cannot be told from the file.
func (s *Service) cleanRows(raw []rawRow) ([]StudentRow, []RowIssue) {
	var issues []RowIssue
	valid := make([]StudentRow, 0, len(raw))
	for _, r := range raw {
		row, reason := s.cleanRow(r)
		if reason != "" {
			issues = append(issues, RowIssue{Sheet: r.sheet, Row: r.row,
				RegisterNumber: strings.TrimSpace(r.fields[colRegister]),
				Email:          strings.TrimSpace(r.fields[colEmail]), Reason: reason})
			continue
		}
		valid = append(valid, row)
	}

	// Rows are grouped transitively by shared email or register number, so a
	// contested row cannot slip through by matching a row that was itself
	// skipped.
	parent := make([]int, len(valid))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	byEmail := map[string]int{}
	byRegister := map[string]int{}
	for i, row := range valid {
		for _, key := range []struct {
			seen map[string]int
			v    string
		}{{byEmail, row.Email}, {byRegister, row.RegisterNumber}} {
			if j, ok := key.seen[key.v]; ok {
				parent[find(i)] = find(j)
			} else {
				key.seen[key.v] = i
			}
		}
	}
	groups := map[int][]int{}
	for i := range valid {
		groups[find(i)] = append(groups[find(i)], i)
	}

	var clean []StudentRow
	for i, row := range valid {
		group := groups[find(i)]
		first := valid[group[0]]
		consistent := true
		for _, j := range group[1:] {
			consistent = consistent && sameAs(first, valid[j]) == first
		}
		switch {
		case consistent && group[0] == i:
			clean = append(clean, row)
		case consistent:
			// An exact repeat: imported once, at its first row.
		default:
			other := first
			if group[0] == i {
				other = valid[group[1]]
			}
			issues = append(issues, RowIssue{Sheet: row.Sheet, Row: row.Row,
				RegisterNumber: row.RegisterNumber, Email: row.Email,
				Reason: fmt.Sprintf("Conflicts with %s row %d (same email or register number, different details)", other.Sheet, other.Row)})
		}
	}
	return clean, issues
}

// sameAs returns row with first's position, so two rows compare equal when
// only where they appear differs.
func sameAs(first, row StudentRow) StudentRow {
	row.Sheet, row.Row = first.Sheet, first.Row
	return row
}

func (s *Service) cleanRow(r rawRow) (StudentRow, string) {
	row := StudentRow{
		Sheet:          r.sheet,
		Row:            r.row,
		RegisterNumber: strings.ToUpper(strings.TrimSpace(r.fields[colRegister])),
		Name:           collapse(r.fields[colName]),
		Department:     collapse(r.fields[colDepartment]),
	}
	email, reason := s.cleanEmail(r.fields[colEmail])
	row.Email = email
	switch {
	case row.RegisterNumber == "":
		return row, "Register number is empty"
	case len(row.RegisterNumber) > maxRegisterLength || !registerPattern.MatchString(row.RegisterNumber):
		return row, "Register number may contain only letters, digits, '/', '.', '_' and '-'"
	case row.Name == "":
		return row, "Student name is empty"
	case utf8.RuneCountInString(row.Name) > maxNameLength:
		return row, "Student name is longer than " + strconv.Itoa(maxNameLength) + " characters"
	case reason != "":
		return row, reason
	case row.Department == "":
		return row, "Department is empty"
	case utf8.RuneCountInString(row.Department) > maxDepartmentLength:
		return row, "Department is longer than " + strconv.Itoa(maxDepartmentLength) + " characters"
	}
	return row, ""
}

// cleanEmail lower-cases an address and checks it can sign in.
func (s *Service) cleanEmail(raw string) (string, string) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", "Email is empty"
	}
	// Workspace addresses are ASCII. users.email compares accent-insensitively,
	// so a non-ASCII address could match another account's in SQL but not here.
	for i := 0; i < len(email); i++ {
		if email[i] >= utf8.RuneSelf {
			return email, "Email may contain only ASCII characters"
		}
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > maxEmailLength {
		return email, "Email is not a valid address"
	}
	if email[strings.LastIndexByte(email, '@')+1:] != s.cfg.AllowedDomain {
		return email, "Email is not on " + s.cfg.AllowedDomain + ", so it could never sign in"
	}
	return email, ""
}

// collapse trims a value and reduces inner runs of whitespace to one space.
func collapse(v string) string {
	return strings.Join(strings.Fields(v), " ")
}

// CreateAdmin adds one administrator.
func (s *Service) CreateAdmin(ctx context.Context, actor assessments.Actor, in AdminInput) (Admin, error) {
	admin, err := s.createAdmin(ctx, in)
	s.audit(ctx, actor, "admin.create", strconv.FormatUint(admin.ID, 10),
		map[string]string{"email": strings.ToLower(strings.TrimSpace(in.Email))}, err)
	return admin, err
}

func (s *Service) createAdmin(ctx context.Context, in AdminInput) (Admin, error) {
	in.Name = collapse(in.Name)
	switch {
	case in.Name == "":
		return Admin{}, fmt.Errorf("%w: a name is required", ErrInvalidInput)
	case utf8.RuneCountInString(in.Name) > maxNameLength:
		return Admin{}, fmt.Errorf("%w: the name is longer than %d characters", ErrInvalidInput, maxNameLength)
	}
	email, reason := s.cleanEmail(in.Email)
	if reason != "" {
		return Admin{}, fmt.Errorf("%w: %s", ErrInvalidInput, reason)
	}
	in.Email = email
	return s.repo.CreateAdmin(ctx, in)
}

func (s *Service) Admins(ctx context.Context) ([]Admin, error) {
	return s.repo.Admins(ctx)
}

func (s *Service) audit(ctx context.Context, actor assessments.Actor, action, entityID string, after any, err error) {
	outcome := "success"
	if err != nil {
		outcome = "failure"
		after = map[string]string{"error": err.Error()}
	}
	if entityID == "0" {
		entityID = ""
	}
	if auditErr := s.auditor.Audit(ctx, assessments.AuditEntry{
		Actor:      actor,
		Action:     action,
		Outcome:    outcome,
		EntityType: "user",
		EntityID:   entityID,
		After:      after,
	}); auditErr != nil {
		s.logger.Error("Audit write failed", zap.String("action", action), zap.Error(auditErr))
	}
}

// Message is the user-facing text for an account error.
func Message(err error) string {
	switch {
	case errors.Is(err, ErrTooLarge):
		return "The file is too large."
	case errors.Is(err, ErrConflict), errors.Is(err, ErrInvalidInput):
		msg := err.Error()
		for _, prefix := range []string{ErrConflict.Error() + ": ", ErrInvalidInput.Error() + ": "} {
			msg = strings.TrimPrefix(msg, prefix)
		}
		return msg
	default:
		return "The action failed."
	}
}
