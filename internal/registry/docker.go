package registry

import (
	"bufio"
	"context"
	"encoding/binary"
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
	CopyLogs(ctx context.Context, containerID string, stdout, stderr io.Writer, opts LogsOptions) error
	FormatLogs(ctx context.Context, containerID string, w io.Writer, opts LogsOptions) error
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
// Use CopyLogs for automatic demultiplexing, or stdcopy.StdCopy manually.
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

// CopyLogs streams container logs to the provided writers with automatic demultiplexing.
// For non-TTY containers, Docker multiplexes stdout/stderr; this method handles that transparently.
// If the container uses a TTY, output goes to stdout only (stderr is ignored in TTY mode).
func (d *dockerClient) CopyLogs(ctx context.Context, containerID string, stdout, stderr io.Writer, opts LogsOptions) error {
	reader, err := d.StreamLogs(ctx, containerID, opts)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Check if container uses TTY
	inspect, err := d.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		slog.Error("container inspect failed during log copy", "container_id", containerID, "error", err)
		return fmt.Errorf("container inspect failed: %w", err)
	}

	if inspect.Config.Tty {
		// TTY mode: no multiplexing, just copy directly
		_, err = io.Copy(stdout, reader)
	} else {
		// Non-TTY: demultiplex stdout/stderr
		_, err = stdcopy.StdCopy(stdout, stderr, reader)
	}

	if err != nil {
		slog.Error("log copy failed", "container_id", containerID, "error", err)
		return fmt.Errorf("log copy failed: %w", err)
	}

	return nil
}

// FormatLogs streams container logs with [STDOUT] and [STDERR] prefixes for each line.
// Output is written to a single writer with prefixes indicating the source stream.
// This is useful for CLI display and TUI integration where stream distinction is needed.
func (d *dockerClient) FormatLogs(ctx context.Context, containerID string, w io.Writer, opts LogsOptions) error {
	reader, err := d.StreamLogs(ctx, containerID, opts)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Check if container uses TTY
	inspect, err := d.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		slog.Error("container inspect failed during log format", "container_id", containerID, "error", err)
		return fmt.Errorf("container inspect failed: %w", err)
	}

	if inspect.Config.Tty {
		// TTY mode: no multiplexing, prefix all as stdout
		return prefixLines(reader, w, "[STDOUT]: ")
	}

	// Non-TTY: demultiplex and prefix each stream
	return demuxAndPrefixLogs(reader, w)
}

// prefixLines reads lines from r and writes them to w with the given prefix.
func prefixLines(r io.Reader, w io.Writer, prefix string) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if _, err := fmt.Fprintf(w, "%s%s\n", prefix, scanner.Text()); err != nil {
			return fmt.Errorf("write failed: %w", err)
		}
	}
	return scanner.Err()
}

// demuxAndPrefixLogs reads Docker's multiplexed log stream and writes prefixed lines.
// Docker's multiplex format: [stream_type(1)][0(3)][size(4)][payload(size)]
func demuxAndPrefixLogs(r io.Reader, w io.Writer) error {
	header := make([]byte, 8)

	for {
		// Read the 8-byte header
		_, err := io.ReadFull(r, header)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read header failed: %w", err)
		}

		// Parse header: byte 0 is stream type, bytes 4-7 are payload size (big endian)
		streamType := header[0]
		size := binary.BigEndian.Uint32(header[4:8])

		// Determine prefix based on stream type
		var prefix string
		switch streamType {
		case 1:
			prefix = "[STDOUT]: "
		case 2:
			prefix = "[STDERR]: "
		default:
			prefix = "[UNKNOWN]: "
		}

		// Read the payload
		payload := make([]byte, size)
		_, err = io.ReadFull(r, payload)
		if err != nil {
			return fmt.Errorf("read payload failed: %w", err)
		}

		// Process payload: split into lines and prefix each
		start := 0
		for i := 0; i < len(payload); i++ {
			if payload[i] == '\n' {
				line := string(payload[start:i])
				if _, err := fmt.Fprintf(w, "%s%s\n", prefix, line); err != nil {
					return fmt.Errorf("write failed: %w", err)
				}
				start = i + 1
			}
		}
		// Handle trailing content without newline
		if start < len(payload) {
			line := string(payload[start:])
			if _, err := fmt.Fprintf(w, "%s%s\n", prefix, line); err != nil {
				return fmt.Errorf("write failed: %w", err)
			}
		}
	}
}
