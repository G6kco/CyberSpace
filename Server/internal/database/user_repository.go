package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/G6kco/CyberSpace/internal/users"
)

// importChunk bounds the rows in one IN list or multi-row INSERT, well under
// MySQL's 65,535 placeholders.
const importChunk = 500

// UserRepository stores accounts created by administrators.
type UserRepository struct {
	db  *sql.DB
	now func() time.Time
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db, now: time.Now}
}

// account is an existing user an imported row matched by email.
type account struct {
	id         uint64
	role       string
	name       string
	register   sql.NullString
	department sql.NullString
}

func (r *UserRepository) ImportStudents(ctx context.Context, rows []users.StudentRow) (users.ImportResult, error) {
	result := users.ImportResult{Skipped: []users.RowIssue{}}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()

	emails := make([]string, len(rows))
	registers := make([]string, len(rows))
	for i, row := range rows {
		emails[i], registers[i] = row.Email, row.RegisterNumber
	}
	// The matched rows are locked, so a concurrent import or sign-in cannot
	// change them between this check and the writes below.
	existing := map[string]account{}
	err = eachChunk(emails, func(placeholders string, args []any) error {
		found, err := tx.QueryContext(ctx,
			`SELECT u.id, u.email, u.role, u.display_name, sp.register_number, sp.department
            FROM users u LEFT JOIN student_profiles sp ON sp.user_id = u.id
            WHERE u.email IN (`+placeholders+`)
            FOR UPDATE`, args...)
		if err != nil {
			return err
		}
		defer found.Close()
		for found.Next() {
			var a account
			var email string
			if err := found.Scan(&a.id, &email, &a.role, &a.name, &a.register, &a.department); err != nil {
				return err
			}
			existing[strings.ToLower(email)] = a
		}
		return found.Err()
	})
	if err != nil {
		return result, err
	}
	owners := map[string]string{}
	err = eachChunk(registers, func(placeholders string, args []any) error {
		found, err := tx.QueryContext(ctx,
			`SELECT sp.register_number, u.email
            FROM student_profiles sp JOIN users u ON u.id = sp.user_id
            WHERE sp.register_number IN (`+placeholders+`)
            FOR UPDATE`, args...)
		if err != nil {
			return err
		}
		defer found.Close()
		for found.Next() {
			var register, email string
			if err := found.Scan(&register, &email); err != nil {
				return err
			}
			owners[register] = strings.ToLower(email)
		}
		return found.Err()
	})
	if err != nil {
		return result, err
	}

	var inserts []users.StudentRow
	for _, row := range rows {
		skip := func(reason string) {
			result.Skipped = append(result.Skipped, users.RowIssue{Sheet: row.Sheet, Row: row.Row,
				RegisterNumber: row.RegisterNumber, Email: row.Email, Reason: reason})
		}
		if owner, taken := owners[row.RegisterNumber]; taken && owner != row.Email {
			skip("Register number already belongs to " + owner)
			continue
		}
		a, found := existing[row.Email]
		switch {
		case !found:
			inserts = append(inserts, row)
		case a.role != "student":
			skip("This email belongs to an administrator")
		case a.name == row.Name && a.register.String == row.RegisterNumber && a.department.String == row.Department:
			result.Unchanged++
		default:
			if err := updateStudent(ctx, tx, a, row); err != nil {
				return result, importError(err)
			}
			result.Updated++
		}
	}
	for start := 0; start < len(inserts); start += importChunk {
		chunk := inserts[start:min(start+importChunk, len(inserts))]
		if err := r.insertStudents(ctx, tx, chunk); err != nil {
			return result, importError(err)
		}
		result.Created += len(chunk)
	}
	if err := tx.Commit(); err != nil {
		return result, importError(err)
	}
	return result, nil
}

// updateStudent leaves the account's status alone: a suspended student
// stays suspended when the list is uploaded again.
func updateStudent(ctx context.Context, tx *sql.Tx, a account, row users.StudentRow) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET display_name = ? WHERE id = ?`, row.Name, a.id); err != nil {
		return err
	}
	if a.register.Valid {
		_, err := tx.ExecContext(ctx,
			`UPDATE student_profiles SET register_number = ?, department = ? WHERE user_id = ?`,
			row.RegisterNumber, row.Department, a.id)
		return err
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO student_profiles (user_id, register_number, department) VALUES (?, ?, ?)`,
		a.id, row.RegisterNumber, row.Department)
	return err
}

func (r *UserRepository) insertStudents(ctx context.Context, tx *sql.Tx, rows []users.StudentRow) error {
	now := r.now()
	args := make([]any, 0, 3*len(rows))
	emails := make([]string, len(rows))
	for i, row := range rows {
		args = append(args, users.NewPublicID(now), row.Email, row.Name)
		emails[i] = row.Email
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (public_id, email, display_name, role, status) VALUES `+
			strings.TrimSuffix(strings.Repeat("(?, ?, ?, 'student', 'active'),", len(rows)), ","),
		args...); err != nil {
		return err
	}

	// Auto-increment IDs of a multi-row insert need not be consecutive, so
	// they are read back by email.
	ids := map[string]uint64{}
	err := eachChunk(emails, func(placeholders string, args []any) error {
		found, err := tx.QueryContext(ctx,
			`SELECT id, email FROM users WHERE email IN (`+placeholders+`)`, args...)
		if err != nil {
			return err
		}
		defer found.Close()
		for found.Next() {
			var id uint64
			var email string
			if err := found.Scan(&id, &email); err != nil {
				return err
			}
			ids[strings.ToLower(email)] = id
		}
		return found.Err()
	})
	if err != nil {
		return err
	}

	args = args[:0]
	for _, row := range rows {
		args = append(args, ids[row.Email], row.RegisterNumber, row.Department)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO student_profiles (user_id, register_number, department) VALUES `+
			strings.TrimSuffix(strings.Repeat("(?, ?, ?),", len(rows)), ","),
		args...)
	return err
}

// importError reports a collision with a concurrent change as a conflict the
// administrator can resolve by uploading again.
func importError(err error) error {
	var mysqlErr *mysql.MySQLError
	// 1062 duplicate key, 1205 lock wait timeout, 1213 deadlock.
	if errors.As(err, &mysqlErr) && (mysqlErr.Number == 1062 || mysqlErr.Number == 1205 || mysqlErr.Number == 1213) {
		return fmt.Errorf("%w: these students changed while the file was being imported; upload it again", users.ErrConflict)
	}
	return err
}

// eachChunk calls fn with an IN list for each chunk of values.
func eachChunk(values []string, fn func(placeholders string, args []any) error) error {
	for start := 0; start < len(values); start += importChunk {
		chunk := values[start:min(start+importChunk, len(values))]
		args := make([]any, len(chunk))
		for i, v := range chunk {
			args[i] = v
		}
		if err := fn(strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ","), args); err != nil {
			return err
		}
	}
	return nil
}

const adminColumns = `id, public_id, display_name, email, status, last_login_at, created_at`

func scanAdmin(row rowScanner) (users.Admin, error) {
	var a users.Admin
	var lastLogin sql.NullTime
	err := row.Scan(&a.ID, &a.PublicID, &a.Name, &a.Email, &a.Status, &lastLogin, &a.CreatedAt)
	a.LastLoginAt = timePtr(lastLogin)
	a.CreatedAt = a.CreatedAt.UTC()
	return a, err
}

func (r *UserRepository) CreateAdmin(ctx context.Context, in users.AdminInput) (users.Admin, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO users (public_id, email, display_name, role, status) VALUES (?, ?, ?, 'admin', 'active')`,
		users.NewPublicID(r.now()), in.Email, in.Name)
	if isDuplicate(err, "uq_users_email") {
		return users.Admin{}, fmt.Errorf("%w: an account with this email already exists", users.ErrConflict)
	}
	if err != nil {
		return users.Admin{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return users.Admin{}, err
	}
	return scanAdmin(r.db.QueryRowContext(ctx, `SELECT `+adminColumns+` FROM users WHERE id = ?`, id))
}

func (r *UserRepository) Admins(ctx context.Context) ([]users.Admin, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+adminColumns+` FROM users WHERE role = 'admin' ORDER BY display_name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	admins := []users.Admin{}
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		admins = append(admins, a)
	}
	return admins, rows.Err()
}
