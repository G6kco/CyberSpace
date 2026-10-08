package labs

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"

	"go.uber.org/zap"
)

// fakeRepo records what the engine asked of storage. Job and instance state
// are kept just far enough to assert the engine's decisions.
type fakeRepo struct {
	jobs     []Job
	target   Target
	loadErr  error
	markErr  error // returned by MarkRunning
	stopErr  error // returned by MarkStopped
	expired  []uint64
	live     map[uint64]bool
	finished map[uint64]string // job ID -> completion status
	retried  []uint64
	failed   []uint64 // FailProvision job IDs
	stopped  []uint64 // RequestStop attempt IDs
	running  []uint64 // MarkRunning instance IDs
	marked   []uint64 // MarkStopped instance IDs
}

func newFakeRepo(jobs ...Job) *fakeRepo {
	return &fakeRepo{
		jobs:     jobs,
		finished: map[uint64]string{},
		target: Target{
			InstanceID: 7,
			Status:     "queued",
			Spec:       Spec{Image: "scenario1"},
			Network:    "macvlan",
			IP:         netip.MustParseAddr("192.168.20.182"),
		},
	}
}

func (f *fakeRepo) RequestProvision(context.Context, uint64, uint64) (uint64, error) {
	return 0, errors.New("not used")
}
func (f *fakeRepo) RequestStop(_ context.Context, attemptID uint64) error {
	f.stopped = append(f.stopped, attemptID)
	return nil
}
func (f *fakeRepo) ClaimJob(context.Context, string, time.Duration) (Job, bool, error) {
	if len(f.jobs) == 0 {
		return Job{}, false, nil
	}
	job := f.jobs[0]
	f.jobs = f.jobs[1:]
	return job, true, nil
}
func (f *fakeRepo) RetryJob(_ context.Context, id uint64, _ time.Duration, _ error) error {
	f.retried = append(f.retried, id)
	return nil
}
func (f *fakeRepo) CompleteJob(_ context.Context, id uint64, status string, _ error) error {
	f.finished[id] = status
	return nil
}
func (f *fakeRepo) FailProvision(_ context.Context, job Job, _ error) error {
	f.failed = append(f.failed, job.ID)
	return nil
}
func (f *fakeRepo) FailStaleJobs(context.Context, time.Duration) (int64, error) { return 0, nil }
func (f *fakeRepo) LoadTarget(context.Context, uint64) (Target, error) {
	return f.target, f.loadErr
}
func (f *fakeRepo) MarkProvisioning(context.Context, uint64) error { return nil }
func (f *fakeRepo) MarkRunning(_ context.Context, id uint64, _ string) error {
	if f.markErr != nil {
		return f.markErr
	}
	f.running = append(f.running, id)
	return nil
}
func (f *fakeRepo) MarkStopped(_ context.Context, id uint64) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	f.marked = append(f.marked, id)
	return nil
}
func (f *fakeRepo) ExpiredAttempts(context.Context) ([]uint64, error) { return f.expired, nil }
func (f *fakeRepo) LiveInstances(context.Context) (map[uint64]bool, error) {
	return f.live, nil
}

type fakeRuntime struct {
	startErr  error
	removeErr error
	started   []Container
	removed   []string
	managed   map[string]uint64
}

func (f *fakeRuntime) Start(_ context.Context, c Container) (string, error) {
	f.started = append(f.started, c)
	return "container-id", f.startErr
}
func (f *fakeRuntime) Remove(_ context.Context, name string) error {
	f.removed = append(f.removed, name)
	return f.removeErr
}
func (f *fakeRuntime) Managed(context.Context) (map[string]uint64, error) {
	return f.managed, nil
}

func newTestEngine(repo *fakeRepo, runtime *fakeRuntime) *Engine {
	return NewEngine(repo, runtime, zap.NewNop(), EngineConfig{})
}

func provisionJob(attempt int) Job {
	return Job{ID: 1, InstanceID: 7, Type: JobProvision, AttemptCount: attempt, MaxAttempts: 3}
}

func stopJob(attempt int) Job {
	return Job{ID: 2, InstanceID: 7, Type: JobStop, AttemptCount: attempt, MaxAttempts: 3}
}

func TestRunOnceWithoutJobs(t *testing.T) {
	found, err := newTestEngine(newFakeRepo(), &fakeRuntime{}).RunOnce(context.Background())
	if found || err != nil {
		t.Fatalf("RunOnce() = %v, %v; want false, nil", found, err)
	}
}

func TestProvisionStartsContainer(t *testing.T) {
	repo, runtime := newFakeRepo(provisionJob(1)), &fakeRuntime{}

	found, err := newTestEngine(repo, runtime).RunOnce(context.Background())
	if !found || err != nil {
		t.Fatalf("RunOnce() = %v, %v; want true, nil", found, err)
	}

	want := Container{
		Name:       "cyberspace-lab-7",
		Image:      "scenario1",
		Network:    "macvlan",
		IP:         netip.MustParseAddr("192.168.20.182"),
		InstanceID: 7,
	}
	if len(runtime.started) != 1 || runtime.started[0] != want {
		t.Fatalf("started = %+v; want [%+v]", runtime.started, want)
	}
	// A leftover container from a crashed run is removed before starting.
	if !slices.Equal(runtime.removed, []string{"cyberspace-lab-7"}) {
		t.Fatalf("removed = %v; want the stale name cleared first", runtime.removed)
	}
	if !slices.Equal(repo.running, []uint64{7}) || repo.finished[1] != "succeeded" {
		t.Fatalf("running = %v, job = %q; want instance 7 running and job succeeded", repo.running, repo.finished[1])
	}
}

func TestProvisionFailureIsRetried(t *testing.T) {
	repo, runtime := newFakeRepo(provisionJob(1)), &fakeRuntime{startErr: errors.New("no such image")}

	if _, err := newTestEngine(repo, runtime).RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() error = nil; want the start error")
	}
	if !slices.Equal(repo.retried, []uint64{1}) || len(repo.failed) != 0 {
		t.Fatalf("retried = %v, failed = %v; want a retry only", repo.retried, repo.failed)
	}
	// The half-created container is removed again so the retry starts clean.
	if len(runtime.removed) != 2 {
		t.Fatalf("removed = %v; want the stale-name removal and the cleanup", runtime.removed)
	}
}

func TestProvisionFinalFailureFailsLab(t *testing.T) {
	repo, runtime := newFakeRepo(provisionJob(3)), &fakeRuntime{startErr: errors.New("no such image")}

	if _, err := newTestEngine(repo, runtime).RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() error = nil; want the start error")
	}
	if !slices.Equal(repo.failed, []uint64{1}) || len(repo.retried) != 0 {
		t.Fatalf("failed = %v, retried = %v; want the lab failed without retry", repo.failed, repo.retried)
	}
}

func TestProvisionLoadFailureIsRetried(t *testing.T) {
	repo, runtime := newFakeRepo(provisionJob(1)), &fakeRuntime{}
	repo.loadErr = errors.New("lab spec has no image")

	if _, err := newTestEngine(repo, runtime).RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() error = nil; want the load error")
	}
	if len(runtime.started) != 0 || !slices.Equal(repo.retried, []uint64{1}) {
		t.Fatalf("started = %v, retried = %v; want no start and a retry", runtime.started, repo.retried)
	}
}

func TestProvisionCancelledWhenStopAlreadyRequested(t *testing.T) {
	repo, runtime := newFakeRepo(provisionJob(1)), &fakeRuntime{}
	repo.target.Status = "stopping"

	if _, err := newTestEngine(repo, runtime).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(runtime.started) != 0 || repo.finished[1] != "cancelled" {
		t.Fatalf("started = %v, job = %q; want nothing started and job cancelled", runtime.started, repo.finished[1])
	}
}

func TestProvisionRemovesContainerWhenStoppedDuringStart(t *testing.T) {
	repo, runtime := newFakeRepo(provisionJob(1)), &fakeRuntime{}
	repo.markErr = ErrStateChanged

	if _, err := newTestEngine(repo, runtime).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(runtime.removed) != 2 || repo.finished[1] != "cancelled" || len(repo.retried) != 0 {
		t.Fatalf("removed = %v, job = %q, retried = %v; want the container removed and job cancelled",
			runtime.removed, repo.finished[1], repo.retried)
	}
}

func TestStopRemovesContainer(t *testing.T) {
	repo, runtime := newFakeRepo(stopJob(1)), &fakeRuntime{}

	if _, err := newTestEngine(repo, runtime).RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if !slices.Equal(runtime.removed, []string{"cyberspace-lab-7"}) ||
		!slices.Equal(repo.marked, []uint64{7}) ||
		repo.finished[2] != "succeeded" {
		t.Fatalf("removed = %v, marked = %v, job = %q", runtime.removed, repo.marked, repo.finished[2])
	}
}

func TestStopFailureRetriesThenFails(t *testing.T) {
	removeErr := errors.New("docker daemon unreachable")

	repo, runtime := newFakeRepo(stopJob(1)), &fakeRuntime{removeErr: removeErr}
	if _, err := newTestEngine(repo, runtime).RunOnce(context.Background()); !errors.Is(err, removeErr) {
		t.Fatalf("RunOnce() error = %v; want %v", err, removeErr)
	}
	if !slices.Equal(repo.retried, []uint64{2}) || len(repo.marked) != 0 {
		t.Fatalf("retried = %v, marked = %v; want a retry and nothing marked", repo.retried, repo.marked)
	}

	repo, runtime = newFakeRepo(stopJob(3)), &fakeRuntime{removeErr: removeErr}
	if _, err := newTestEngine(repo, runtime).RunOnce(context.Background()); !errors.Is(err, removeErr) {
		t.Fatalf("RunOnce() error = %v; want %v", err, removeErr)
	}
	if repo.finished[2] != "failed" || len(repo.retried) != 0 {
		t.Fatalf("job = %q, retried = %v; want the job failed", repo.finished[2], repo.retried)
	}
}

func TestUnknownJobTypeFails(t *testing.T) {
	repo := newFakeRepo(Job{ID: 9, InstanceID: 7, Type: "reset", AttemptCount: 1, MaxAttempts: 3})

	if _, err := newTestEngine(repo, &fakeRuntime{}).RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() error = nil; want unsupported job type")
	}
	if repo.finished[9] != "failed" {
		t.Fatalf("job = %q; want failed", repo.finished[9])
	}
}

func TestSweepStopsExpiredLabsAndRemovesOrphans(t *testing.T) {
	repo := newFakeRepo()
	repo.expired = []uint64{11, 12}
	repo.live = map[uint64]bool{7: true}
	runtime := &fakeRuntime{managed: map[string]uint64{
		"cyberspace-lab-7": 7, // live: kept
		"cyberspace-lab-8": 8, // instance stopped or failed: removed
	}}

	newTestEngine(repo, runtime).Sweep(context.Background())

	if !slices.Equal(repo.stopped, []uint64{11, 12}) {
		t.Fatalf("stopped = %v; want both expired attempts", repo.stopped)
	}
	if !slices.Equal(runtime.removed, []string{"cyberspace-lab-8"}) {
		t.Fatalf("removed = %v; want only the orphan", runtime.removed)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		newTestEngine(newFakeRepo(), &fakeRuntime{}).Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}
