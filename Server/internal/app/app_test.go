package app

import (
	"context"
	"testing"

	"github.com/G6kco/CyberSpace/internal/config"
	"go.uber.org/zap"
)

type testDatabase struct{}

func (testDatabase) PingContext(context.Context) error { return nil }

func TestNew(t *testing.T) {
	cfg := &config.Config{AppEnv: "test"}
	db := testDatabase{}
	log := zap.NewNop()

	application, err := New(cfg, db, log)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if application.Config != cfg {
		t.Error("Config was not retained")
	}
	if application.DB != db {
		t.Error("DB was not retained")
	}
	if application.Logger != log {
		t.Error("Logger was not retained")
	}
}

func TestNewRejectsMissingDependencies(t *testing.T) {
	cfg := &config.Config{}
	db := testDatabase{}
	log := zap.NewNop()

	tests := []struct {
		name string
		cfg  *config.Config
		db   Database
		log  *zap.Logger
		want string
	}{
		{name: "config", db: db, log: log, want: "application config is required"},
		{name: "database", cfg: cfg, log: log, want: "application database is required"},
		{name: "logger", cfg: cfg, db: db, want: "application logger is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.cfg, test.db, test.log)
			if err == nil || err.Error() != test.want {
				t.Fatalf("New() error = %v, want %q", err, test.want)
			}
		})
	}
}
