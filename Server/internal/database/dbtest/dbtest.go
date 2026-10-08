// Package dbtest opens the disposable MySQL database the integration tests
// share. It is imported only by tests.
package dbtest

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/G6kco/CyberSpace/migrations"
	"github.com/go-sql-driver/mysql"
	migrate "github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// EnvVar names a disposable MySQL database. Its tables are emptied by every
// test, so the database name must end in "_test".
const EnvVar = "CYBERSPACE_TEST_DATABASE_URL"

// lockName serializes tests across packages: `go test ./...` runs packages
// in parallel processes, and they all empty the same tables.
const lockName = "cyberspace_integration_tests"

// tables lists every table tests write to, children before parents, since
// foreign keys restrict deleting a parent that still has rows.
var tables = []string{
	"orchestration_jobs", "ip_allocations", "lab_instance_services", "lab_instances",
	"network_addresses", "network_pools", "docker_hosts",
	"audit_logs", "attempt_events", "attempt_question_results", "question_submissions",
	"attempt_questions", "attempts", "assessment_access_grants",
	"answer_validators", "assessment_questions", "assessment_versions", "assessments", "levels",
	"lab_template_versions", "lab_templates",
	"student_profiles", "auth_sessions", "oauth_login_flows", "users",
}

var (
	migrateOnce  sync.Once
	migrateError error
)

// Open migrates the test database, waits for other test packages to finish
// with it, empties it, and returns a pool closed when the test ends. It skips
// the test when EnvVar is unset.
func Open(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv(EnvVar)
	if dsn == "" {
		t.Skipf("set %s to a disposable *_test MySQL database to run database tests", EnvVar)
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvVar, err)
	}
	if !strings.HasSuffix(cfg.DBName, "_test") {
		t.Fatalf("refusing to empty database %q: %s must name a database ending in _test", cfg.DBName, EnvVar)
	}

	migrateOnce.Do(func() { migrateError = migrateUp(*cfg) })
	if migrateError != nil {
		t.Fatalf("migrate test database: %v", migrateError)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// The named lock belongs to one connection, held until the test ends.
	ctx := context.Background()
	lockConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("reserve lock connection: %v", err)
	}
	var acquired sql.NullInt64
	if err := lockConn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 120)`, lockName).Scan(&acquired); err != nil || acquired.Int64 != 1 {
		t.Fatalf("acquire test database lock: %v (result %v)", err, acquired)
	}
	t.Cleanup(func() {
		_, _ = lockConn.ExecContext(ctx, `SELECT RELEASE_LOCK(?)`, lockName)
		_ = lockConn.Close()
	})

	// A published assessment and its current version reference each other,
	// so the cycle is broken before either is deleted.
	if _, err := db.Exec(`UPDATE assessments SET status = 'draft', current_version_id = NULL`); err != nil {
		t.Fatalf("unlink assessment versions: %v", err)
	}
	for _, table := range tables {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("empty %s: %v", table, err)
		}
	}
	return db
}

func migrateUp(cfg mysql.Config) error {
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		_ = db.Close()
		return err
	}
	driver, err := migratemysql.WithInstance(db, &migratemysql.Config{})
	if err != nil {
		_ = db.Close()
		return err
	}
	migrator, err := migrate.NewWithInstance("iofs", source, "mysql", driver)
	if err != nil {
		_ = driver.Close()
		return err
	}
	defer migrator.Close()

	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
