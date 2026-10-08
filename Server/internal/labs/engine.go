package labs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EngineConfig tunes the engine. Zero values are replaced by defaults.
type EngineConfig struct {
	// WorkerID identifies this process in orchestration_jobs.locked_by.
	WorkerID string
	// Workers is how many jobs run at once. A test start provisions every
	// student together, so containers are started in parallel.
	Workers int
	// PollInterval is how long an idle worker waits before looking again.
	PollInterval time.Duration
	// SweepInterval is how often expired labs and orphan containers are
	// cleaned up.
	SweepInterval time.Duration
	// JobLease is how long a claimed job may run before another worker may
	// assume its owner died and take it over.
	JobLease time.Duration
	// OperationTimeout bounds one Docker start or remove.
	OperationTimeout time.Duration
	// RetryDelay is multiplied by the attempt number between retries.
	RetryDelay time.Duration
}

func (c *EngineConfig) applyDefaults() {
	if c.WorkerID == "" {
		c.WorkerID = "api"
	}
	if c.Workers <= 0 {
		c.Workers = 4
	}
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	}
	if c.SweepInterval <= 0 {
		c.SweepInterval = 30 * time.Second
	}
	if c.JobLease <= 0 {
		c.JobLease = 5 * time.Minute
	}
	if c.OperationTimeout <= 0 {
		c.OperationTimeout = 60 * time.Second
	}
	if c.RetryDelay <= 0 {
		c.RetryDelay = 5 * time.Second
	}
}

// Engine runs provision and stop jobs against a container Runtime.
type Engine struct {
	repo    Repository
	runtime Runtime
	logger  *zap.Logger
	cfg     EngineConfig
}

func NewEngine(repo Repository, runtime Runtime, logger *zap.Logger, cfg EngineConfig) *Engine {
	cfg.applyDefaults()
	return &Engine{repo: repo, runtime: runtime, logger: logger, cfg: cfg}
}

// Run processes jobs and sweeps until ctx is cancelled. It blocks, so callers
// start it in its own goroutine; it returns once every worker has finished
// its current job.
func (e *Engine) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for range e.cfg.Workers {
		wg.Go(func() { e.work(ctx) })
	}
	wg.Go(func() { e.sweepLoop(ctx) })
	wg.Wait()
}

func (e *Engine) work(ctx context.Context) {
	for {
		found, err := e.RunOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			e.logger.Error("Lab job failed", zap.Error(err))
		}
		if found {
			continue // more work may be waiting
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(e.cfg.PollInterval):
		}
	}
}

// RunOnce claims and runs one job, reporting whether there was one.
func (e *Engine) RunOnce(ctx context.Context) (bool, error) {
	job, found, err := e.repo.ClaimJob(ctx, e.cfg.WorkerID, e.cfg.JobLease)
	if err != nil || !found {
		return false, err
	}

	// A claimed job is finished even during shutdown, so a container is
	// never left running without its state being recorded.
	jobCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*e.cfg.OperationTimeout)
	defer cancel()

	switch job.Type {
	case JobProvision:
		return true, e.provision(jobCtx, job)
	case JobStop:
		return true, e.stop(jobCtx, job)
	default:
		// Not retryable: no code exists to run it.
		err := fmt.Errorf("unsupported job type %q", job.Type)
		return true, errors.Join(err, e.repo.CompleteJob(jobCtx, job.ID, "failed", err))
	}
}

func (e *Engine) provision(ctx context.Context, job Job) error {
	target, err := e.repo.LoadTarget(ctx, job.InstanceID)
	if err != nil {
		return e.provisionFailed(ctx, job, fmt.Errorf("load lab %d: %w", job.InstanceID, err))
	}
	// A stop requested before this job ran leaves nothing to start.
	if target.Status != "queued" && target.Status != "provisioning" {
		return e.repo.CompleteJob(ctx, job.ID, "cancelled", nil)
	}
	if err := e.repo.MarkProvisioning(ctx, job.InstanceID); err != nil {
		return e.notStarted(ctx, job, err)
	}

	name := ContainerName(job.InstanceID)
	opCtx, cancel := context.WithTimeout(ctx, e.cfg.OperationTimeout)
	defer cancel()

	// A container left by a run that crashed halfway holds the name and the
	// IP, so it is removed first.
	containerID, err := "", e.runtime.Remove(opCtx, name)
	if err == nil {
		containerID, err = e.runtime.Start(opCtx, Container{
			Name:       name,
			Image:      target.Spec.Image,
			Network:    target.Network,
			IP:         target.IP,
			InstanceID: job.InstanceID,
		})
	}
	if err == nil {
		err = e.repo.MarkRunning(opCtx, job.InstanceID, containerID)
	}
	if err == nil {
		e.logger.Info("Lab started",
			zap.Uint64("instance", job.InstanceID),
			zap.String("image", target.Spec.Image),
			zap.String("ip", target.IP.String()),
		)
		return e.repo.CompleteJob(ctx, job.ID, "succeeded", nil)
	}

	// Undo the start with a fresh timeout, since opCtx may be what ran out.
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), e.cfg.OperationTimeout)
	defer cleanupCancel()
	if cleanupErr := e.runtime.Remove(cleanupCtx, name); cleanupErr != nil {
		err = fmt.Errorf("%w (cleanup also failed: %v)", err, cleanupErr)
	}
	return e.notStarted(ctx, job, err)
}

// notStarted settles a provision job whose container is not running.
func (e *Engine) notStarted(ctx context.Context, job Job, err error) error {
	if errors.Is(err, ErrStateChanged) {
		// The stop job owns the lab now.
		return e.repo.CompleteJob(ctx, job.ID, "cancelled", nil)
	}
	return e.provisionFailed(ctx, job, err)
}

func (e *Engine) provisionFailed(ctx context.Context, job Job, cause error) error {
	if job.AttemptCount < job.MaxAttempts {
		delay := e.cfg.RetryDelay * time.Duration(job.AttemptCount)
		return errors.Join(cause, e.repo.RetryJob(ctx, job.ID, delay, cause))
	}
	return errors.Join(cause, e.repo.FailProvision(ctx, job, cause))
}

func (e *Engine) stop(ctx context.Context, job Job) error {
	opCtx, cancel := context.WithTimeout(ctx, e.cfg.OperationTimeout)
	defer cancel()

	err := e.runtime.Remove(opCtx, ContainerName(job.InstanceID))
	if err == nil {
		err = e.repo.MarkStopped(opCtx, job.InstanceID)
	}
	if err == nil {
		e.logger.Info("Lab stopped", zap.Uint64("instance", job.InstanceID))
		return e.repo.CompleteJob(ctx, job.ID, "succeeded", nil)
	}

	if job.AttemptCount < job.MaxAttempts {
		delay := e.cfg.RetryDelay * time.Duration(job.AttemptCount)
		return errors.Join(err, e.repo.RetryJob(ctx, job.ID, delay, err))
	}
	// The instance stays 'stopping' and keeps its address; the sweep sees
	// the attempt is over and requeues this job, so a stop that failed while
	// Docker was down completes once Docker is back.
	return errors.Join(err, e.repo.CompleteJob(ctx, job.ID, "failed", err))
}

func (e *Engine) sweepLoop(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.SweepInterval)
	defer ticker.Stop()

	e.Sweep(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.Sweep(ctx)
		}
	}
}

// Sweep queues stops for labs whose attempt is over and removes containers
// that belong to no live lab. Failures are only logged; the next sweep
// retries.
func (e *Engine) Sweep(ctx context.Context) {
	if failed, err := e.repo.FailStaleJobs(ctx, e.cfg.JobLease); err != nil && ctx.Err() == nil {
		e.logger.Error("Failing stale lab jobs failed", zap.Error(err))
	} else if failed > 0 {
		e.logger.Warn("Failed lab jobs whose worker stopped responding", zap.Int64("jobs", failed))
	}

	expired, err := e.repo.ExpiredAttempts(ctx)
	if err != nil && ctx.Err() == nil {
		e.logger.Error("Listing expired labs failed", zap.Error(err))
	}
	for _, attemptID := range expired {
		if err := e.repo.RequestStop(ctx, attemptID); err != nil && ctx.Err() == nil {
			e.logger.Error("Stopping expired lab failed", zap.Uint64("attempt", attemptID), zap.Error(err))
		}
	}

	e.removeOrphans(ctx)
}

func (e *Engine) removeOrphans(ctx context.Context) {
	// Containers are listed before live instances: a lab provisioned between
	// the two reads is then live in the second, so it is never removed.
	managed, err := e.runtime.Managed(ctx)
	if err != nil {
		if ctx.Err() == nil {
			e.logger.Error("Listing lab containers failed", zap.Error(err))
		}
		return
	}
	if len(managed) == 0 {
		return
	}
	live, err := e.repo.LiveInstances(ctx)
	if err != nil {
		if ctx.Err() == nil {
			e.logger.Error("Listing live labs failed", zap.Error(err))
		}
		return
	}

	for name, instanceID := range managed {
		if live[instanceID] {
			continue
		}
		if err := e.runtime.Remove(ctx, name); err != nil {
			if ctx.Err() == nil {
				e.logger.Error("Removing orphan lab container failed", zap.String("container", name), zap.Error(err))
			}
			continue
		}
		e.logger.Info("Removed orphan lab container", zap.String("container", name))
	}
}
