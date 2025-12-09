package registry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/docker/docker/api/types/container"
	imageTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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
	StreamLogs(ctx context.Context, containerID string, opts LogsOptions) (io.ReadCloser, error)
	Exec(ctx context.Context, containerID string, cmd []string, opts ExecOptions) (*ExecResult, error)
	ImageExists(ctx context.Context, image string) (bool, error)
	ImageInspect(ctx context.Context, image string) (*LocalImageInfo, error)
}

// LocalImageInfo contains information about a local Docker image.
type LocalImageInfo struct {
	ID           string   // Image ID (sha256:...)
	RepoTags     []string // Repository tags (e.g., ["alpine:latest"])
	RepoDigests  []string // Repository digests
	Size         int64    // Image size in bytes
	Architecture string   // Image architecture (e.g., "amd64")
	Os           string   // Operating system (e.g., "linux")
}

// LogsOptions configures container log streaming.
type LogsOptions struct {
	Follow     bool   // Follow log output (like tail -f)
	Tail       string // Number of lines to show from the end (e.g., "100", "all")
	Since      string // Show logs since timestamp (e.g., "2021-01-01T00:00:00Z") or relative (e.g., "1h")
	Until      string // Show logs until timestamp or relative
	Timestamps bool   // Show timestamps in output
	Stdout     bool   // Include stdout
	Stderr     bool   // Include stderr
}

// ExecOptions configures a container exec operation.
type ExecOptions struct {
	User         string   // User to run the command as (e.g., "root", "1000:1000")
	WorkingDir   string   // Working directory inside the container
	Env          []string // Environment variables (KEY=value format)
	Privileged   bool     // Run with extended privileges
	Tty          bool     // Allocate a pseudo-TTY
	AttachStdin  bool     // Attach stdin
	AttachStdout bool     // Attach stdout (default: true if not specified)
	AttachStderr bool     // Attach stderr (default: true if not specified)
}

// ExecResult contains the result of an exec operation.
type ExecResult struct {
	ExitCode int    // Exit code of the executed command
	Stdout   []byte // Captured stdout
	Stderr   []byte // Captured stderr
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

// StreamLogs returns an io.ReadCloser for streaming container logs.
// The caller is responsible for closing the returned reader.
// Note: Docker multiplexes stdout/stderr with an 8-byte header per frame when TTY is disabled.
// Use stdcopy.StdCopy from github.com/docker/docker/pkg/stdcopy to demultiplex if needed.
func (d *dockerClient) StreamLogs(ctx context.Context, containerID string, opts LogsOptions) (io.ReadCloser, error) {
	slog.Debug("streaming container logs",
		"container_id", containerID,
		"follow", opts.Follow,
		"tail", opts.Tail,
	)

	dockerOpts := container.LogsOptions{
		ShowStdout: opts.Stdout,
		ShowStderr: opts.Stderr,
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
		Tail:       opts.Tail,
		Since:      opts.Since,
		Until:      opts.Until,
	}

	// Default to both stdout and stderr if neither specified
	if !dockerOpts.ShowStdout && !dockerOpts.ShowStderr {
		dockerOpts.ShowStdout = true
		dockerOpts.ShowStderr = true
	}

	reader, err := d.cli.ContainerLogs(ctx, containerID, dockerOpts)
	if err != nil {
		slog.Error("container logs failed", "container_id", containerID, "error", err)
		return nil, fmt.Errorf("container logs failed: %w", err)
	}

	return reader, nil
}

// Exec executes a command inside a running container and returns the result.
// This is equivalent to `docker exec` and captures both stdout and stderr.
func (d *dockerClient) Exec(ctx context.Context, containerID string, cmd []string, opts ExecOptions) (*ExecResult, error) {
	slog.Debug("executing command in container",
		"container_id", containerID,
		"cmd", cmd,
		"user", opts.User,
		"working_dir", opts.WorkingDir,
	)

	// Default to attaching stdout/stderr if not explicitly set
	attachStdout := opts.AttachStdout
	attachStderr := opts.AttachStderr
	if !attachStdout && !attachStderr && !opts.AttachStdin {
		attachStdout = true
		attachStderr = true
	}

	execConfig := container.ExecOptions{
		User:         opts.User,
		Privileged:   opts.Privileged,
		Tty:          opts.Tty,
		AttachStdin:  opts.AttachStdin,
		AttachStdout: attachStdout,
		AttachStderr: attachStderr,
		Cmd:          cmd,
		WorkingDir:   opts.WorkingDir,
		Env:          opts.Env,
	}

	// Create the exec instance
	execID, err := d.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		slog.Error("exec create failed", "container_id", containerID, "error", err)
		return nil, fmt.Errorf("exec create failed: %w", err)
	}

	// Attach to the exec instance to capture output
	resp, err := d.cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{
		Tty: opts.Tty,
	})
	if err != nil {
		slog.Error("exec attach failed", "container_id", containerID, "error", err)
		return nil, fmt.Errorf("exec attach failed: %w", err)
	}
	defer resp.Close()

	// Read stdout and stderr
	var stdout, stderr bytes.Buffer
	if opts.Tty {
		// TTY mode: all output goes to stdout
		_, err = io.Copy(&stdout, resp.Reader)
	} else {
		// Non-TTY: demultiplex stdout/stderr using Docker's stdcopy
		_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	}
	if err != nil && err != io.EOF {
		slog.Error("exec read output failed", "container_id", containerID, "error", err)
		return nil, fmt.Errorf("exec read output failed: %w", err)
	}

	// Inspect the exec to get the exit code
	inspectResp, err := d.cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		slog.Error("exec inspect failed", "container_id", containerID, "error", err)
		return nil, fmt.Errorf("exec inspect failed: %w", err)
	}

	result := &ExecResult{
		ExitCode: inspectResp.ExitCode,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}

	slog.Debug("exec completed",
		"container_id", containerID,
		"exit_code", result.ExitCode,
		"stdout_len", len(result.Stdout),
		"stderr_len", len(result.Stderr),
	)

	return result, nil
}

// ImageExists checks if an image exists in the local Docker daemon.
func (d *dockerClient) ImageExists(ctx context.Context, image string) (bool, error) {
	slog.Debug("checking if image exists locally", "image", image)

	_, _, err := d.cli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		if client.IsErrNotFound(err) {
			slog.Debug("image not found locally", "image", image)
			return false, nil
		}
		slog.Error("image inspect failed", "image", image, "error", err)
		return false, fmt.Errorf("image inspect failed: %w", err)
	}

	slog.Debug("image exists locally", "image", image)
	return true, nil
}

// ImageInspect returns detailed information about a local Docker image.
// Returns nil if the image does not exist locally.
func (d *dockerClient) ImageInspect(ctx context.Context, image string) (*LocalImageInfo, error) {
	slog.Debug("inspecting local image", "image", image)

	inspect, _, err := d.cli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		if client.IsErrNotFound(err) {
			slog.Debug("image not found locally", "image", image)
			return nil, nil
		}
		slog.Error("image inspect failed", "image", image, "error", err)
		return nil, fmt.Errorf("image inspect failed: %w", err)
	}

	info := &LocalImageInfo{
		ID:           inspect.ID,
		RepoTags:     inspect.RepoTags,
		RepoDigests:  inspect.RepoDigests,
		Size:         inspect.Size,
		Architecture: inspect.Architecture,
		Os:           inspect.Os,
	}

	slog.Debug("local image inspected",
		"image", image,
		"id", info.ID,
		"tags", info.RepoTags,
		"size", info.Size,
	)

	return info, nil
}
