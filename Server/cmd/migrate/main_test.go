package main

import (
	"errors"
	"os"
	"testing"

	"github.com/G6kco/CyberSpace/migrations"
	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing command"},
		{name: "unsupported command", args: []string{"down"}},
		{name: "too many arguments", args: []string{"up", "1"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := run(test.args)
			if err == nil {
				t.Fatal("run() error = nil, want a usage error")
			}
			if err.Error() != "usage: cyberspace-migrate up" {
				t.Fatalf("run() error = %q, want %q", err, "usage: cyberspace-migrate up")
			}
		})
	}
}

func TestMigrationDSNEnablesMultiStatements(t *testing.T) {
	rawDSN := "user:password@tcp(localhost:3306)/cyberspace?parseTime=true"

	preparedDSN, err := migrationDSN(rawDSN)
	if err != nil {
		t.Fatalf("migrationDSN() error = %v", err)
	}

	parsed, err := mysql.ParseDSN(preparedDSN)
	if err != nil {
		t.Fatalf("parse prepared DSN: %v", err)
	}
	if !parsed.MultiStatements {
		t.Error("MultiStatements = false, want true")
	}
	if !parsed.ParseTime {
		t.Error("ParseTime = false, want the original setting to be preserved")
	}
	if parsed.DBName != "cyberspace" {
		t.Errorf("DBName = %q, want %q", parsed.DBName, "cyberspace")
	}
}

func TestMigrationDSNRejectsInvalidDSN(t *testing.T) {
	if _, err := migrationDSN("user:password@tcp(localhost:3306"); err == nil {
		t.Fatal("migrationDSN() error = nil, want an invalid DSN error")
	}
}

func TestEmbeddedMigrationsAreSequential(t *testing.T) {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("open embedded migrations: %v", err)
	}
	defer source.Close()

	version, err := source.First()
	if err != nil {
		t.Fatalf("read first migration: %v", err)
	}

	count := 1
	first := version
	last := version

	for {
		next, err := source.Next(last)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			t.Fatalf("read migration after version %d: %v", last, err)
		}
		if next != last+1 {
			t.Fatalf("migration after version %d is %d, want %d", last, next, last+1)
		}

		last = next
		count++
	}

	if first != 1 {
		t.Errorf("first migration version = %d, want 1", first)
	}
	// Versions are derived rather than hardcoded so that adding a migration
	// does not require editing this test. The loop above proves each step is
	// exactly +1; this asserts the resulting range has no gaps or duplicates.
	if count != int(last) {
		t.Errorf(
			"embedded migration count = %d, want %d (versions 1..%d with no gaps)",
			count, last, last,
		)
	}
}
