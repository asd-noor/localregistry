// Package registry provides a Docker CLI wrapper and registry utilities.
//
// This package offers a Go interface for interacting with Docker, leveraging the official Docker Go SDK.
// It is designed for extensibility, testability, and robust error handling.
package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

const (
	DefaultRegistryImage = "registry:3"
	DefaultRegistryPort  = "5000"
	DefaultContainerName = "registry"
)

// RegistryConfig holds configuration for a local Docker registry.
type RegistryConfig struct {
	Image         string
	Name          string
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
	if config.Name == "" {
		config.Name = DefaultContainerName
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
	if err := r.docker.PullImage(ctx, r.config.Image); err != nil {
		return fmt.Errorf("failed to pull registry image: %w", err)
	}

	containerConfig, hostConfig := r.buildContainerConfig()

	id, err := r.docker.CreateContainer(ctx, containerConfig, hostConfig, r.config.Name)
	if err != nil {
		return fmt.Errorf("failed to create registry container: %w", err)
	}
	r.containerID = id

	if err := r.docker.StartContainer(ctx, r.containerID); err != nil {
		return fmt.Errorf("failed to start registry container: %w", err)
	}

	return nil
}

// Stop stops the registry container.
func (r *Registry) Stop(ctx context.Context) error {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return err
		}
	}
	timeout := 10
	if err := r.docker.StopContainer(ctx, r.containerID, &timeout); err != nil {
		return fmt.Errorf("failed to stop registry container: %w", err)
	}
	return nil
}

// Remove stops and removes the registry container.
func (r *Registry) Remove(ctx context.Context, force bool) error {
	if r.containerID == "" {
		if err := r.findContainer(ctx); err != nil {
			return err
		}
	}
	if err := r.docker.RemoveContainer(ctx, r.containerID, force); err != nil {
		return fmt.Errorf("failed to remove registry container: %w", err)
	}
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
	return fmt.Sprintf("localhost:%s", r.config.HostPort)
}

func (r *Registry) findContainer(ctx context.Context) error {
	containers, err := r.docker.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	for _, c := range containers {
		for _, name := range c.Names {
			if strings.TrimPrefix(name, "/") == r.config.Name {
				r.containerID = c.ID
				return nil
			}
		}
	}
	return fmt.Errorf("registry container %q not found", r.config.Name)
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
