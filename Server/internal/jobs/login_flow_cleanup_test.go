package jobs

import (
      "context"
      "errors"
      "sync/atomic"
      "testing"
      "testing/synctest"
      "time"

      "go.uber.org/zap"
      "go.uber.org/zap/zaptest/observer"
)

type fakeCleaner struct {
      calls atomic.Int64
      err   error
}

func (f *fakeCleaner) DeleteExpiredFlows(context.Context, time.Duration) (int64, error) {
      f.calls.Add(1)
      return 3, f.err
}

func TestRunCleansAtStartupAndEveryInterval(t *testing.T) {
      synctest.Test(t, func(t *testing.T) {
              cleaner := &fakeCleaner{}
              job := NewLoginFlowCleanup(cleaner, zap.NewNop(), time.Hour, time.Hour)
              ctx, cancel := context.WithCancel(t.Context())
              done := make(chan struct{})

              go func() {
                      job.Run(ctx)
                      close(done)
              }()

              synctest.Wait() // let Run reach its first select
              if got := cleaner.calls.Load(); got != 1 {
                      t.Fatalf("calls after startup = %d, want 1", got)
              }

              time.Sleep(time.Hour) // fake clock: returns instantly
              synctest.Wait()
              if got := cleaner.calls.Load(); got != 2 {
                      t.Fatalf("calls after one interval = %d, want 2", got)
              }

              cancel()
              <-done // Run must return once cancelled
      })
}

func TestRunOnceLogsFailure(t *testing.T) {
      core, logs := observer.New(zap.DebugLevel)
      job := NewLoginFlowCleanup(
              &fakeCleaner{err: errors.New("database offline")},
              zap.New(core), time.Hour, time.Hour,
      )

      job.runOnce(context.Background())

      if logs.FilterMessage("Login flow cleanup failed").Len() != 1 {
              t.Error("failure was not logged")
      }
}