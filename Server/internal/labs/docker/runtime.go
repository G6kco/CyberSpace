// Package docker is the labs.Runtime backed by the local Docker daemon. Each
// lab is one container on an existing macvlan network with a fixed IP, so
// students on the LAN reach it directly.
package docker

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"github.com/G6kco/CyberSpace/internal/labs"
)

// Labels mark the containers this platform owns, so orphan cleanup never
// touches anything else running on the host.
const (
	labelManaged  = "cyberspace.managed"
	labelInstance = "cyberspace.lab_instance_id"
)

type Runtime struct {
	cli *client.Client
}

// New connects to the Docker daemon named by DOCKER_HOST, or the platform
// default socket when it is unset, and checks that it answers.
func New(ctx context.Context) (*Runtime, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	if _, err := cli.Ping(ctx); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("reach docker daemon: %w", err)
	}
	return &Runtime{cli: cli}, nil
}

func (r *Runtime) Close() error {
	return r.cli.Close()
}

// Start creates and starts the container and returns its ID. If starting
// fails, the created container is left behind; the engine removes it.
func (r *Runtime) Start(ctx context.Context, c labs.Container) (string, error) {
	created, err := r.cli.ContainerCreate(ctx,
		&container.Config{
			Image: c.Image,
			Labels: map[string]string{
				labelManaged:  "true",
				labelInstance: strconv.FormatUint(c.InstanceID, 10),
			},
		},
		nil,
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				c.Network: {IPAMConfig: &network.EndpointIPAMConfig{IPv4Address: c.IP.String()}},
			},
		},
		nil,
		c.Name,
	)
	if err != nil {
		return "", fmt.Errorf("create container %s: %w", c.Name, err)
	}

	if err := r.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start container %s: %w", c.Name, err)
	}
	return created.ID, nil
}

// Remove stops and deletes the container in one step. A container that does
// not exist is skipped, so it is safe to call at any time, as often as needed.
func (r *Runtime) Remove(ctx context.Context, name string) error {
	err := r.cli.ContainerRemove(ctx, name, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true, // images such as MySQL create anonymous volumes
	})
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("remove container %s: %w", name, err)
	}
	return nil
}

// Managed returns every container carrying the platform label, running or
// not, keyed by name.
func (r *Runtime) Managed(ctx context.Context) (map[string]uint64, error) {
	list, err := r.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", labelManaged+"=true")),
	})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	managed := make(map[string]uint64, len(list))
	for _, c := range list {
		instanceID, err := strconv.ParseUint(c.Labels[labelInstance], 10, 64)
		if err != nil || len(c.Names) == 0 {
			continue // not one of ours in a form we understand; leave it alone
		}
		// The API reports names with a leading '/'.
		managed[strings.TrimPrefix(c.Names[0], "/")] = instanceID
	}
	return managed, nil
}
