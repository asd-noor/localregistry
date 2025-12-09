// Package registry provides a Docker CLI wrapper and registry utilities.
//
// This package offers a Go interface for interacting with Docker, leveraging the official Docker Go SDK.
// It is designed for extensibility, testability, and robust error handling.
package registry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

const (
	DefaultRegistryImage = "registry:3"
	DefaultRegistryPort  = "5000"
	DefaultContainerName = "local-registry"
)

// RegistryConfig holds configuration for a local Docker registry.
type RegistryConfig struct {
	Image         string
	HostName      string
	ContainerName string
	HostPort      string
	ContainerPort string
	DataVolume    string
	RestartPolicy string
	TLS           *TLSConfig
	Auth          *AuthConfig
	Env           []string
}

// TLSConfig holds TLS certificate configuration.
type TLSConfig struct {
	CertPath string
	KeyPath  string
}

// AuthConfig holds basic authentication configuration.
type AuthConfig struct {
	HtpasswdPath string
	Realm        string
}

// GarbageCollectResult contains the result of a garbage collection operation.
type GarbageCollectResult struct {
	ExitCode int    // Exit code from the gc command
	Output   string // Stdout from the gc command
	Stderr   string // Stderr from the gc command
}

// RegistryInfo contains detailed information about a registry container.
type RegistryInfo struct {
	ContainerID   string   // Docker container ID
	ContainerName string   // Container name
	Image         string   // Docker image used
	Status        string   // Container status (running, exited, etc.)
	Running       bool     // Whether the container is running
	StartedAt     string   // Container start time
	Address       string   // Registry address (host:port)
	Ports         []string // Port bindings
	Mounts        []string // Volume mounts
}

// Registry manages a local Docker distribution registry container.
type Registry struct {
	config      RegistryConfig
	docker      DockerClient
	containerID string
}

// NewRegistry creates a new Registry instance with the provided configuration.
func NewRegistry(docker DockerClient, config RegistryConfig) *Registry {
	if config.Image == "" {
		config.Image = DefaultRegistryImage
	}
	if config.ContainerName == "" {
		config.ContainerName = DefaultContainerName
	}
	if config.HostPort == "" {
		config.HostPort = DefaultRegistryPort
	}
	if config.ContainerPort == "" {
		config.ContainerPort = DefaultRegistryPort
	}
	if config.RestartPolicy == "" {
		config.RestartPolicy = "always"
	}
	return &Registry{
		config: config,
		docker: docker,
	}
}

// Start pulls the registry image (if needed) and starts the registry container.
func (r *Registry) Start(ctx context.Context) error {
	slog.Debug("starting registry container",
		"image", r.config.Image,
		"container_name", r.config.ContainerName,
		"host_port", r.config.HostPort,
	)

	if err := r.docker.PullImage(ctx, r.config.Image); err != nil {
		slog.Error("failed to pull registry image", "image", r.config.Image, "error", err)
		return fmt.Errorf("failed to pull registry image: %w", err)
	}

	slog.Debug("pulled registry image", "image", r.config.Image)

	containerConfig, hostConfig := r.buildContainerConfig()

	id, err := r.docker.CreateContainer(ctx, containerConfig, hostConfig, r.config.ContainerName)
	if err != nil {
		slog.Error("failed to create registry container", "error", err)
		return fmt.Errorf("failed to create registry container: %w", err)
	}
	r.containerID = id

	slog.Debug("created registry container", "container_id", id)

	if err := r.docker.StartContainer(ctx, r.containerID); err != nil {
		slog.Error("failed to start registry container", "container_id", r.containerID, "error", err)
		return fmt.Errorf("failed to start registry container: %w", err)
	}

	slog.Info("registry container started",
		"container_id", r.containerID,
		"address", r.Address(),
	)

	return nil
}

// Stop stops the registry container.
func (r *Registry) Stop(ctx context.Context) error {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return err
		}
	}

	slog.Debug("stopping registry container", "container_id", r.containerID)

	timeout := 10
	if err := r.docker.StopContainer(ctx, r.containerID, &timeout); err != nil {
		slog.Error("failed to stop registry container", "container_id", r.containerID, "error", err)
		return fmt.Errorf("failed to stop registry container: %w", err)
	}

	slog.Info("registry container stopped", "container_id", r.containerID)
	return nil
}

// Remove stops and removes the registry container.
func (r *Registry) Remove(ctx context.Context, force bool) error {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return err
		}
	}

	slog.Debug("removing registry container", "container_id", r.containerID, "force", force)

	if err := r.docker.RemoveContainer(ctx, r.containerID, force); err != nil {
		slog.Error("failed to remove registry container", "container_id", r.containerID, "error", err)
		return fmt.Errorf("failed to remove registry container: %w", err)
	}

	slog.Info("registry container removed", "container_id", r.containerID)
	r.containerID = ""
	return nil
}

// Status returns the running status of the registry container.
func (r *Registry) Status(ctx context.Context) (string, error) {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return "not found", nil
		}
	}
	info, err := r.docker.InspectContainer(ctx, r.containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect registry container: %w", err)
	}
	return info.State.Status, nil
}

// ContainerID returns the current container ID.
func (r *Registry) ContainerID() string {
	return r.containerID
}

// Address returns the registry address (host:port).
func (r *Registry) Address() string {
	return fmt.Sprintf("%s:%s", r.config.HostName, r.config.HostPort)
}

// GarbageCollect runs the registry garbage collection to reclaim disk space.
// This executes `bin/registry garbage-collect --delete-untagged /etc/distribution/config.yml`
// inside the registry container.
func (r *Registry) GarbageCollect(ctx context.Context, deleteUntagged bool) (*GarbageCollectResult, error) {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return nil, err
		}
	}

	slog.Debug("running garbage collection", "container_id", r.containerID, "delete_untagged", deleteUntagged)

	cmd := []string{"bin/registry", "garbage-collect", "/etc/distribution/config.yml"}
	if deleteUntagged {
		cmd = []string{"bin/registry", "garbage-collect", "--delete-untagged", "/etc/distribution/config.yml"}
	}

	result, err := r.docker.Exec(ctx, r.containerID, cmd, ExecOptions{})
	if err != nil {
		slog.Error("garbage collection exec failed", "container_id", r.containerID, "error", err)
		return nil, fmt.Errorf("garbage collection failed: %w", err)
	}

	gcResult := &GarbageCollectResult{
		ExitCode: result.ExitCode,
		Output:   string(result.Stdout),
		Stderr:   string(result.Stderr),
	}

	if result.ExitCode != 0 {
		slog.Error("garbage collection failed",
			"container_id", r.containerID,
			"exit_code", result.ExitCode,
			"stderr", gcResult.Stderr,
		)
		return gcResult, fmt.Errorf("garbage collection exited with code %d: %s", result.ExitCode, gcResult.Stderr)
	}

	slog.Info("garbage collection completed", "container_id", r.containerID)
	return gcResult, nil
}

// RemoveRepository removes a repository directory from the registry storage.
// This is used after deleting all tags via the API to fully clean up the repository.
// The path is relative to the registry storage root (e.g., "myrepo" or "org/myrepo").
func (r *Registry) RemoveRepository(ctx context.Context, repository string) error {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return err
		}
	}

	// Sanitize repository path to prevent directory traversal
	if strings.Contains(repository, "..") {
		return fmt.Errorf("invalid repository path: contains '..'")
	}
	repository = strings.TrimPrefix(repository, "/")
	if repository == "" {
		return fmt.Errorf("invalid repository path: empty")
	}

	repoPath := fmt.Sprintf("/var/lib/registry/docker/registry/v2/repositories/%s", repository)

	slog.Debug("removing repository directory",
		"container_id", r.containerID,
		"repository", repository,
		"path", repoPath,
	)

	result, err := r.docker.Exec(ctx, r.containerID, []string{"rm", "-rf", repoPath}, ExecOptions{})
	if err != nil {
		slog.Error("remove repository exec failed", "container_id", r.containerID, "repository", repository, "error", err)
		return fmt.Errorf("failed to remove repository %q: %w", repository, err)
	}

	if result.ExitCode != 0 {
		errMsg := strings.TrimSpace(string(result.Stderr))
		slog.Error("remove repository failed",
			"container_id", r.containerID,
			"repository", repository,
			"exit_code", result.ExitCode,
			"stderr", errMsg,
		)
		return fmt.Errorf("failed to remove repository %q: exit code %d: %s", repository, result.ExitCode, errMsg)
	}

	slog.Info("repository directory removed", "repository", repository)
	return nil
}

// RepositoryExists checks if a repository directory exists in the registry storage.
func (r *Registry) RepositoryExists(ctx context.Context, repository string) (bool, error) {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return false, err
		}
	}

	repository = strings.TrimPrefix(repository, "/")
	repoPath := fmt.Sprintf("/var/lib/registry/docker/registry/v2/repositories/%s", repository)

	result, err := r.docker.Exec(ctx, r.containerID, []string{"test", "-d", repoPath}, ExecOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to check repository existence: %w", err)
	}

	return result.ExitCode == 0, nil
}

// Exec executes an arbitrary command inside the registry container.
// This is a lower-level method for advanced use cases.
func (r *Registry) Exec(ctx context.Context, cmd []string, opts ExecOptions) (*ExecResult, error) {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return nil, err
		}
	}

	return r.docker.Exec(ctx, r.containerID, cmd, opts)
}

// Restart restarts the registry container.
func (r *Registry) Restart(ctx context.Context) error {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return err
		}
	}

	slog.Debug("restarting registry container", "container_id", r.containerID)

	timeout := 10
	if err := r.docker.StopContainer(ctx, r.containerID, &timeout); err != nil {
		slog.Error("failed to stop registry container for restart", "container_id", r.containerID, "error", err)
		return fmt.Errorf("failed to stop registry container: %w", err)
	}

	if err := r.docker.StartContainer(ctx, r.containerID); err != nil {
		slog.Error("failed to start registry container after restart", "container_id", r.containerID, "error", err)
		return fmt.Errorf("failed to start registry container: %w", err)
	}

	slog.Info("registry container restarted", "container_id", r.containerID)
	return nil
}

// Info returns detailed information about the registry container.
func (r *Registry) Info(ctx context.Context) (*RegistryInfo, error) {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return nil, err
		}
	}

	inspect, err := r.docker.InspectContainer(ctx, r.containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect registry container: %w", err)
	}

	info := &RegistryInfo{
		ContainerID:   r.containerID,
		ContainerName: r.config.ContainerName,
		Image:         inspect.Config.Image,
		Status:        inspect.State.Status,
		Running:       inspect.State.Running,
		StartedAt:     inspect.State.StartedAt,
		Address:       r.Address(),
	}

	// Extract port bindings
	for port, bindings := range inspect.NetworkSettings.Ports {
		for _, binding := range bindings {
			info.Ports = append(info.Ports, fmt.Sprintf("%s:%s->%s", binding.HostIP, binding.HostPort, port))
		}
	}

	// Extract mounts
	for _, mount := range inspect.Mounts {
		info.Mounts = append(info.Mounts, fmt.Sprintf("%s:%s", mount.Source, mount.Destination))
	}

	return info, nil
}

func (r *Registry) findContainer(ctx context.Context) error {
	containers, err := r.docker.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	for _, c := range containers {
		for _, name := range c.Names {
			if strings.TrimPrefix(name, "/") == r.config.ContainerName {
				r.containerID = c.ID
				return nil
			}
		}
	}
	return fmt.Errorf("registry container %q not found", r.config.ContainerName)
}

func (r *Registry) buildContainerConfig() (*container.Config, *container.HostConfig) {
	containerPort := nat.Port(r.config.ContainerPort + "/tcp")

	env := append([]string{}, r.config.Env...)

	binds := []string{}
	if r.config.DataVolume != "" {
		binds = append(binds, fmt.Sprintf("%s:/var/lib/registry", r.config.DataVolume))
	}

	if r.config.TLS != nil {
		env = append(env,
			fmt.Sprintf("REGISTRY_HTTP_TLS_CERTIFICATE=%s", r.config.TLS.CertPath),
			fmt.Sprintf("REGISTRY_HTTP_TLS_KEY=%s", r.config.TLS.KeyPath),
		)
	}

	if r.config.Auth != nil {
		env = append(env,
			"REGISTRY_AUTH=htpasswd",
			fmt.Sprintf("REGISTRY_AUTH_HTPASSWD_PATH=%s", r.config.Auth.HtpasswdPath),
			fmt.Sprintf("REGISTRY_AUTH_HTPASSWD_REALM=%s", r.config.Auth.Realm),
		)
	}

	containerConfig := &container.Config{
		Image:        r.config.Image,
		Env:          env,
		ExposedPorts: nat.PortSet{containerPort: struct{}{}},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			containerPort: []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: r.config.HostPort},
			},
		},
		Binds: binds,
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyMode(r.config.RestartPolicy),
		},
	}

	return containerConfig, hostConfig
}
