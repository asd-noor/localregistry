package registry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/docker/docker/api/types/container"
	imageTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// DockerClient abstracts Docker operations for testability and extensibility.
type DockerClient interface {
	PullImage(ctx context.Context, image string) error
	RunContainer(ctx context.Context, image string, cmd []string) (string, error)
	CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeout *int) error
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	InspectContainer(ctx context.Context, containerID string) (*container.InspectResponse, error)
	ListContainers(ctx context.Context, all bool) ([]container.Summary, error)
}

// dockerClient implements DockerClient using the official Docker Go SDK.
type dockerClient struct {
	cli *client.Client
}

// NewDockerClient creates a new DockerClient using environment configuration and API version negotiation.
func NewDockerClient() (DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		slog.Error("failed to create docker client", "error", err)
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	slog.Debug("docker client created")
	return &dockerClient{cli: cli}, nil
}

// PullImage pulls a Docker image from a registry.
func (d *dockerClient) PullImage(ctx context.Context, image string) (err error) {
	slog.Debug("pulling docker image", "image", image)

	reader, err := d.cli.ImagePull(ctx, image, imageTypes.PullOptions{}) // image.PullOptions from types/image
	if err != nil {
		slog.Error("image pull failed", "image", image, "error", err)
		return fmt.Errorf("pull failed: %w", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close failed: %w", closeErr)
		}
	}()
	_, _ = io.Copy(os.Stdout, reader) // Optionally parse output for progress

	slog.Debug("docker image pulled", "image", image)
	return nil
}

// RunContainer creates and starts a container from the specified image and command.
func (d *dockerClient) RunContainer(ctx context.Context, image string, cmd []string) (string, error) {
	resp, err := d.cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   cmd,
	}, nil, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("container create failed: %w", err)
	}
	if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("container start failed: %w", err)
	}
	return resp.ID, nil
}

// CreateContainer creates a container with the specified configuration.
func (d *dockerClient) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) (string, error) {
	resp, err := d.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("container create failed: %w", err)
	}
	return resp.ID, nil
}

// StartContainer starts an existing container by ID.
func (d *dockerClient) StartContainer(ctx context.Context, containerID string) error {
	if err := d.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start failed: %w", err)
	}
	return nil
}

// StopContainer stops a running container with an optional timeout.
func (d *dockerClient) StopContainer(ctx context.Context, containerID string, timeout *int) error {
	if err := d.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: timeout}); err != nil {
		return fmt.Errorf("container stop failed: %w", err)
	}
	return nil
}

// RemoveContainer removes a container by ID.
func (d *dockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if err := d.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: force, RemoveVolumes: true}); err != nil {
		return fmt.Errorf("container remove failed: %w", err)
	}
	return nil
}

// InspectContainer returns detailed information about a container.
func (d *dockerClient) InspectContainer(ctx context.Context, containerID string) (*container.InspectResponse, error) {
	resp, err := d.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("container inspect failed: %w", err)
	}
	return &resp, nil
}

// ListContainers returns a list of containers.
func (d *dockerClient) ListContainers(ctx context.Context, all bool) ([]container.Summary, error) {
	containers, err := d.cli.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, fmt.Errorf("container list failed: %w", err)
	}
	return containers, nil
}
