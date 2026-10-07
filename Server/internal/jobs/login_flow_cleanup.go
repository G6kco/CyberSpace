// Package jobs runs periodic background maintenance alongside the API.
package jobs

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// FlowCleaner is the storage behavior the cleanup job needs.
type FlowCleaner interface {
	DeleteExpiredFlows(ctx context.Context, grace time.Duration) (int64, error)
}

// LoginFlowCleanup periodically deletes OAuth login flows that can no longer
// be used, so the table does not grow with every login attempt.
type LoginFlowCleanup struct {
	cleaner FlowCleaner
	logger *zap.Logger
	interval time.Duration
	grace time.Duration
}

func NewLoginFlowCleanup(
	cleaner FlowCleaner,
	logger *zap.Logger,
	interval, grace time.Duration,
) *LoginFlowCleanup {
	return &LoginFlowCleanup{
		cleaner: cleaner,
		logger: logger,
		interval: interval,
		grace: grace,
	}
}

// Run cleans once immediately and then on every interval until ctx is
// cancelled. It blocks, so callers start it in its own goroutine.
func (r *LoginFlowCleanup) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	
	r.runOnce(ctx)
	for {
		select {
		case <- ctx.Done():
			return
		case <- ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *LoginFlowCleanup) runOnce(ctx context.Context) {
	deleted, err := r.cleaner.DeleteExpiredFlows(ctx, r.grace)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		
		r.logger.Error("Login flow cleanup failed", zap.Error(err))
		return
	}
	
	if deleted > 0 {
		r.logger.Info("Deleted Expired login flows", zap.Int64("Deleted", deleted))
	}
}


