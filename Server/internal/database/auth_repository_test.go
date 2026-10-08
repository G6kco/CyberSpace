package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/G6kco/CyberSpace/internal/auth"
	"github.com/G6kco/CyberSpace/internal/database/dbtest"
)

// openTestDatabase returns the shared test database, emptied; it skips the
// test unless dbtest.EnvVar is set.
func openTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	return dbtest.Open(t)
}

var userSequence int

func insertUser(t *testing.T, db *sql.DB, email, status string) auth.User {
	t.Helper()

	userSequence++
	user := auth.User{
		PublicID: fmt.Sprintf("01TESTUSER%016d", userSequence),
		Name:     "Test " + email,
		Email:    email,
		Role:     "student",
	}
	result, err := db.Exec(
		`INSERT INTO users (public_id, email, display_name, role, status)
		VALUES (?, ?, ?, ?, ?)`,
		user.PublicID, user.Email, user.Name, user.Role, status,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read user id: %v", err)
	}
	user.ID = uint64(id)
	return user
}

func exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func hash(value string) [32]byte {
	return sha256.Sum256([]byte(value))
}

func newFlow(state, browser string, expiresIn time.Duration) auth.LoginFlow {
	return auth.LoginFlow{
		StateHash:   hash(state),
		BrowserHash: hash(browser),
		Nonce:       "nonce-" + state,
		Verifier:    "verifier-" + state,
		ExpiresAt:   time.Now().UTC().Add(expiresIn),
	}
}

func TestConsumeFlowReturnsSavedFlowOnce(t *testing.T) {
	db := openTestDatabase(t)
	repo := NewAuthRepository(db)
	ctx := context.Background()
	flow := newFlow("state", "browser", 5*time.Minute)

	if err := repo.SaveFlow(ctx, flow); err != nil {
		t.Fatalf("SaveFlow() error = %v", err)
	}

	consumed, err := repo.ConsumeFlow(ctx, flow.StateHash, flow.BrowserHash)
	if err != nil {
		t.Fatalf("ConsumeFlow() error = %v", err)
	}
	if consumed.Nonce != flow.Nonce || consumed.Verifier != flow.Verifier {
		t.Errorf("ConsumeFlow() = %+v, want nonce %q and verifier %q", consumed, flow.Nonce, flow.Verifier)
	}

	var consumedAt sql.NullTime
	if err := db.QueryRow(
		"SELECT consumed_at FROM oauth_login_flows WHERE state_hash = ?", flow.StateHash[:],
	).Scan(&consumedAt); err != nil {
		t.Fatalf("read consumed_at: %v", err)
	}
	if !consumedAt.Valid {
		t.Error("consumed_at was not recorded")
	}

	if _, err := repo.ConsumeFlow(ctx, flow.StateHash, flow.BrowserHash); !errors.Is(err, auth.ErrInvalidFlow) {
		t.Fatalf("replayed ConsumeFlow() error = %v, want %v", err, auth.ErrInvalidFlow)
	}
}

func TestConsumeFlowRefusals(t *testing.T) {
	tests := []struct {
		name      string
		flow      auth.LoginFlow
		stateHash [32]byte
		browser   [32]byte
	}{
		{
			name:      "unknown state",
			flow:      newFlow("state", "browser", 5*time.Minute),
			stateHash: hash("other-state"),
			browser:   hash("browser"),
		},
		{
			name:      "expired flow",
			flow:      newFlow("state", "browser", -time.Second),
			stateHash: hash("state"),
			browser:   hash("browser"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := NewAuthRepository(openTestDatabase(t))
			ctx := context.Background()
			if err := repo.SaveFlow(ctx, test.flow); err != nil {
				t.Fatalf("SaveFlow() error = %v", err)
			}

			if _, err := repo.ConsumeFlow(ctx, test.stateHash, test.browser); !errors.Is(err, auth.ErrInvalidFlow) {
				t.Fatalf("ConsumeFlow() error = %v, want %v", err, auth.ErrInvalidFlow)
			}
		})
	}
}

func TestConsumeFlowWrongBrowserLeavesFlowUsable(t *testing.T) {
	repo := NewAuthRepository(openTestDatabase(t))
	ctx := context.Background()
	flow := newFlow("state", "browser", 5*time.Minute)
	if err := repo.SaveFlow(ctx, flow); err != nil {
		t.Fatalf("SaveFlow() error = %v", err)
	}

	if _, err := repo.ConsumeFlow(ctx, flow.StateHash, hash("attacker")); !errors.Is(err, auth.ErrInvalidFlow) {
		t.Fatalf("ConsumeFlow() with wrong browser error = %v, want %v", err, auth.ErrInvalidFlow)
	}
	if _, err := repo.ConsumeFlow(ctx, flow.StateHash, flow.BrowserHash); err != nil {
		t.Fatalf("ConsumeFlow() from the right browser error = %v", err)
	}
}

func TestConsumeFlowConcurrentCallbacksSucceedOnce(t *testing.T) {
	repo := NewAuthRepository(openTestDatabase(t))
	ctx := context.Background()
	flow := newFlow("state", "browser", 5*time.Minute)
	if err := repo.SaveFlow(ctx, flow); err != nil {
		t.Fatalf("SaveFlow() error = %v", err)
	}

	const callers = 8
	errs := make(chan error, callers)
	var start sync.WaitGroup
	start.Add(1)
	for range callers {
		go func() {
			start.Wait()
			_, err := repo.ConsumeFlow(ctx, flow.StateHash, flow.BrowserHash)
			errs <- err
		}()
	}
	start.Done()

	successes := 0
	for range callers {
		err := <-errs
		switch {
		case err == nil:
			successes++
		case !errors.Is(err, auth.ErrInvalidFlow):
			t.Errorf("concurrent ConsumeFlow() error = %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("%d concurrent callbacks succeeded, want exactly 1", successes)
	}
}

func TestFindOrBindUserBindsOnFirstLogin(t *testing.T) {
	db := openTestDatabase(t)
	repo := NewAuthRepository(db)
	stored := insertUser(t, db, "student@bitsathy.ac.in", "active")

	user, err := repo.FindOrBindUser(context.Background(), "subject-1", stored.Email)
	if err != nil {
		t.Fatalf("FindOrBindUser() error = %v", err)
	}
	if user != stored {
		t.Errorf("FindOrBindUser() = %+v, want %+v", user, stored)
	}

	var subject sql.NullString
	var lastLogin sql.NullTime
	if err := db.QueryRow(
		"SELECT google_subject, last_login_at FROM users WHERE id = ?", stored.ID,
	).Scan(&subject, &lastLogin); err != nil {
		t.Fatalf("read user: %v", err)
	}
	if subject.String != "subject-1" {
		t.Errorf("google_subject = %q, want %q", subject.String, "subject-1")
	}
	if !lastLogin.Valid {
		t.Error("last_login_at was not recorded")
	}
}

func TestFindOrBindUserMatchesBoundSubjectAfterEmailChange(t *testing.T) {
	db := openTestDatabase(t)
	repo := NewAuthRepository(db)
	stored := insertUser(t, db, "student@bitsathy.ac.in", "active")
	exec(t, db, "UPDATE users SET google_subject = 'subject-1' WHERE id = ?", stored.ID)

	user, err := repo.FindOrBindUser(context.Background(), "subject-1", "renamed@bitsathy.ac.in")
	if err != nil {
		t.Fatalf("FindOrBindUser() error = %v", err)
	}
	if user.ID != stored.ID {
		t.Errorf("FindOrBindUser() user = %d, want %d", user.ID, stored.ID)
	}
}

func TestFindOrBindUserMatchesEmailCaseInsensitively(t *testing.T) {
	db := openTestDatabase(t)
	repo := NewAuthRepository(db)
	stored := insertUser(t, db, "student@bitsathy.ac.in", "active")

	user, err := repo.FindOrBindUser(context.Background(), "subject-1", "Student@BITSATHY.ac.in")
	if err != nil {
		t.Fatalf("FindOrBindUser() error = %v", err)
	}
	if user.ID != stored.ID {
		t.Errorf("FindOrBindUser() user = %d, want %d", user.ID, stored.ID)
	}
}

func TestFindOrBindUserRefusals(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		subject string // already bound subject, if any
		email   string // email Google reports
	}{
		{name: "unknown email", status: "active", email: "stranger@bitsathy.ac.in"},
		{name: "suspended user", status: "suspended", email: "student@bitsathy.ac.in"},
		{name: "archived user", status: "archived", email: "student@bitsathy.ac.in"},
		{name: "bound to another Google account", status: "active", subject: "subject-other", email: "student@bitsathy.ac.in"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openTestDatabase(t)
			repo := NewAuthRepository(db)
			stored := insertUser(t, db, "student@bitsathy.ac.in", test.status)
			if test.subject != "" {
				exec(t, db, "UPDATE users SET google_subject = ? WHERE id = ?", test.subject, stored.ID)
			}

			_, err := repo.FindOrBindUser(context.Background(), "subject-1", test.email)
			if !errors.Is(err, auth.ErrAccessDenied) {
				t.Fatalf("FindOrBindUser() error = %v, want %v", err, auth.ErrAccessDenied)
			}

			var subject sql.NullString
			if err := db.QueryRow("SELECT google_subject FROM users WHERE id = ?", stored.ID).Scan(&subject); err != nil {
				t.Fatalf("read subject: %v", err)
			}
			if subject.String != test.subject {
				t.Errorf("google_subject = %q after refusal, want %q", subject.String, test.subject)
			}
		})
	}
}

func TestResolveSessionReturnsUser(t *testing.T) {
	db := openTestDatabase(t)
	repo := NewAuthRepository(db)
	ctx := context.Background()
	stored := insertUser(t, db, "student@bitsathy.ac.in", "active")

	if err := repo.CreateSession(ctx, stored.ID, hash("token"), time.Now().UTC().Add(4*time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	user, err := repo.ResolveSession(ctx, hash("token"))
	if err != nil {
		t.Fatalf("ResolveSession() error = %v", err)
	}
	if user != stored {
		t.Errorf("ResolveSession() = %+v, want %+v", user, stored)
	}
}

func TestResolveSessionRefusals(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(t *testing.T, db *sql.DB, repo *AuthRepository, user auth.User)
		lookup  string
	}{
		{name: "unknown token", lookup: "other-token"},
		{
			name: "expired session",
			arrange: func(t *testing.T, db *sql.DB, _ *AuthRepository, _ auth.User) {
				// The expiry CHECK requires expires_at > created_at, so both move.
				exec(t, db, `UPDATE auth_sessions
					SET created_at = UTC_TIMESTAMP(6) - INTERVAL 5 HOUR,
					    expires_at = UTC_TIMESTAMP(6) - INTERVAL 1 HOUR`)
			},
		},
		{
			name: "revoked session",
			arrange: func(t *testing.T, _ *sql.DB, repo *AuthRepository, _ auth.User) {
				if err := repo.RevokeSession(context.Background(), hash("token")); err != nil {
					t.Fatalf("RevokeSession() error = %v", err)
				}
			},
		},
		{
			name: "suspended user",
			arrange: func(t *testing.T, db *sql.DB, _ *AuthRepository, user auth.User) {
				exec(t, db, "UPDATE users SET status = 'suspended' WHERE id = ?", user.ID)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openTestDatabase(t)
			repo := NewAuthRepository(db)
			ctx := context.Background()
			user := insertUser(t, db, "student@bitsathy.ac.in", "active")
			if err := repo.CreateSession(ctx, user.ID, hash("token"), time.Now().UTC().Add(4*time.Hour)); err != nil {
				t.Fatalf("CreateSession() error = %v", err)
			}
			if test.arrange != nil {
				test.arrange(t, db, repo, user)
			}

			lookup := test.lookup
			if lookup == "" {
				lookup = "token"
			}
			if _, err := repo.ResolveSession(ctx, hash(lookup)); !errors.Is(err, auth.ErrUnauthenticated) {
				t.Fatalf("ResolveSession() error = %v, want %v", err, auth.ErrUnauthenticated)
			}
		})
	}
}

func TestRevokeSessionRecordsLogoutOnce(t *testing.T) {
	db := openTestDatabase(t)
	repo := NewAuthRepository(db)
	ctx := context.Background()
	user := insertUser(t, db, "student@bitsathy.ac.in", "active")
	if err := repo.CreateSession(ctx, user.ID, hash("token"), time.Now().UTC().Add(4*time.Hour)); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	tokenHash := hash("token")
	readRevocation := func() (time.Time, string) {
		t.Helper()
		var revokedAt sql.NullTime
		var reason sql.NullString
		if err := db.QueryRow(
			"SELECT revoked_at, revocation_reason FROM auth_sessions WHERE token_hash = ?", tokenHash[:],
		).Scan(&revokedAt, &reason); err != nil {
			t.Fatalf("read revocation: %v", err)
		}
		if !revokedAt.Valid {
			t.Fatal("revoked_at was not recorded")
		}
		return revokedAt.Time, reason.String
	}

	if err := repo.RevokeSession(ctx, hash("token")); err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
	firstRevokedAt, reason := readRevocation()
	if reason != "user_logout" {
		t.Errorf("revocation_reason = %q, want user_logout", reason)
	}

	if err := repo.RevokeSession(ctx, hash("token")); err != nil {
		t.Fatalf("second RevokeSession() error = %v", err)
	}
	if again, _ := readRevocation(); !again.Equal(firstRevokedAt) {
		t.Errorf("second logout moved revoked_at from %v to %v", firstRevokedAt, again)
	}

	if err := repo.RevokeSession(ctx, hash("unknown")); err != nil {
		t.Errorf("RevokeSession() for an unknown token error = %v, want nil", err)
	}
}

// TestDatabaseFailuresAreNotRefusals needs no MySQL: a closed pool fails every
// call, which must surface as a fault rather than as a refusal sentinel.
func TestDatabaseFailuresAreNotRefusals(t *testing.T) {
	db, err := NewMySQL("user:password@tcp(127.0.0.1:1)/cyberspace_test")
	if err != nil {
		t.Fatalf("NewMySQL() error = %v", err)
	}
	_ = db.Close()
	repo := NewAuthRepository(db)
	ctx := context.Background()

	calls := map[string]func() error{
		"SaveFlow": func() error { return repo.SaveFlow(ctx, newFlow("state", "browser", time.Minute)) },
		"ConsumeFlow": func() error {
			_, err := repo.ConsumeFlow(ctx, hash("state"), hash("browser"))
			return err
		},
		"FindOrBindUser": func() error {
			_, err := repo.FindOrBindUser(ctx, "subject", "student@bitsathy.ac.in")
			return err
		},
		"CreateSession": func() error { return repo.CreateSession(ctx, 1, hash("token"), time.Now().Add(time.Hour)) },
		"ResolveSession": func() error {
			_, err := repo.ResolveSession(ctx, hash("token"))
			return err
		},
		"RevokeSession": func() error { return repo.RevokeSession(ctx, hash("token")) },
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			if err == nil {
				t.Fatal("error = nil, want a database error")
			}
			for _, sentinel := range []error{auth.ErrInvalidFlow, auth.ErrAccessDenied, auth.ErrUnauthenticated} {
				if errors.Is(err, sentinel) {
					t.Errorf("database fault %v was classified as %v", err, sentinel)
				}
			}
		})
	}
}
