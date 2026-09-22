// Package app defines the application's explicitly injected dependencies.
package app

import (
	"context"
	"errors"

	"github.com/G6kco/CyberSpace/internal/config"
	"go.uber.org/zap"
)

// Database is the database behavior required by the application today.
// Add query methods here as repositories are introduced. Keeping this as an
// interface lets tests provide a small fake instead of relying on global state.
type Database interface {
	PingContext(context.Context) error
}

// App contains the process-wide dependencies passed into application layers.
type App struct {
	Config *config.Config
	DB     Database
	Logger *zap.Logger
}

// New creates a dependency container and rejects incomplete startup wiring.
func New(cfg *config.Config, db Database, logger *zap.Logger) (*App, error) {
	switch {
	case cfg == nil:
		return nil, errors.New("application config is required")
	case db == nil:
		return nil, errors.New("application database is required")
	case logger == nil:
		return nil, errors.New("application logger is required")
	default:
		return &App{
			Config: cfg,
			DB:     db,
			Logger: logger,
		}, nil
	}
}
