// Package labs starts and removes the Docker container behind each attempt's
// lab. Requests are written to the database as orchestration jobs; the Engine
// claims and runs them, so a container is never started from a request
// handler and a crashed run is retried.
package labs

import (
	"context"
	"errors"
	"net/netip"
	"strconv"
	"time"
)

var (
	// ErrNoFreeAddress means no online host has both spare capacity and an
	// address that is neither reserved nor allocated to an active lab.
	ErrNoFreeAddress = errors.New("no free lab IP address")
	// ErrLabActive means the attempt already has a lab that is not stopped.
	ErrLabActive = errors.New("attempt already has an active lab")
	// ErrStateChanged means the instance left the state the engine expected,
	// usually because a stop was requested while it was being provisioned.
	ErrStateChanged = errors.New("lab instance changed state")
)

// Job types and the service name used for the single container of a lab.
const (
	JobProvision = "provision"
	JobStop      = "stop"

	TargetService = "target"
)

// Job is one claimed orchestration job.
type Job struct {
	ID           uint64
	InstanceID   uint64
	Type         string
	AttemptCount int
	MaxAttempts  int
}

// Target is everything needed to start one lab's container.
type Target struct {
	InstanceID uint64
	Status     string
	Spec       Spec
	Network    string
	IP         netip.Addr
}

// Container is what the Runtime is asked to start.
type Container struct {
	Name       string
	Image      string
	Network    string
	IP         netip.Addr
	InstanceID uint64
}

// Repository is the storage the engine needs.
type Repository interface {
	// RequestProvision creates a queued lab for the attempt, allocates it an
	// address, and queues its provision job, all in one transaction.
	RequestProvision(ctx context.Context, attemptID, templateVersionID uint64) (instanceID uint64, err error)
	// RequestStop queues the removal of the attempt's active lab. It does
	// nothing when the attempt has no active lab, so it is safe to repeat.
	RequestStop(ctx context.Context, attemptID uint64) error

	ClaimJob(ctx context.Context, workerID string, lease time.Duration) (Job, bool, error)
	RetryJob(ctx context.Context, jobID uint64, delay time.Duration, cause error) error
	// CompleteJob finishes a running job; cause, when not nil, is recorded.
	CompleteJob(ctx context.Context, jobID uint64, status string, cause error) error
	// FailStaleJobs fails jobs whose worker died on their final attempt.
	FailStaleJobs(ctx context.Context, lease time.Duration) (int64, error)
	// FailProvision marks the job and its lab failed and frees the address.
	FailProvision(ctx context.Context, job Job, cause error) error

	LoadTarget(ctx context.Context, instanceID uint64) (Target, error)
	MarkProvisioning(ctx context.Context, instanceID uint64) error
	MarkRunning(ctx context.Context, instanceID uint64, containerID string) error
	// MarkStopped records the container as removed and frees the address.
	MarkStopped(ctx context.Context, instanceID uint64) error

	// ExpiredAttempts lists attempts whose lab is still active although the
	// attempt has ended or passed its deadline.
	ExpiredAttempts(ctx context.Context) ([]uint64, error)
	// LiveInstances lists instances whose container may legitimately exist.
	LiveInstances(ctx context.Context) (map[uint64]bool, error)
}

// Runtime is the container platform the engine drives.
type Runtime interface {
	Start(ctx context.Context, c Container) (containerID string, err error)
	// Remove deletes the named container; a missing container is not an error.
	Remove(ctx context.Context, name string) error
	// Managed returns the instance ID of every container this platform created.
	Managed(ctx context.Context) (map[string]uint64, error)
}

// ContainerName is the Docker name of an instance's container. Naming by
// instance rather than by student keeps names unique across retakes.
func ContainerName(instanceID uint64) string {
	return "cyberspace-lab-" + strconv.FormatUint(instanceID, 10)
}
