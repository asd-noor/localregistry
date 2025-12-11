package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// Transport represents a skopeo image transport type.
type Transport string

const (
	// TransportDocker is the docker:// transport for registry images.
	TransportDocker Transport = "docker://"
	// TransportDockerDaemon is the docker-daemon: transport for local Docker daemon.
	TransportDockerDaemon Transport = "docker-daemon:"
)

// SkopeoConfig holds configuration for skopeo operations.
type SkopeoConfig struct {
	// SkopeoPath is the path to the skopeo binary (default: "skopeo").
	SkopeoPath string

	// InsecurePolicy skips TLS verification for all operations.
	InsecurePolicy bool
}

// Credentials holds authentication credentials for a registry.
type Credentials struct {
	Username string
	Password string
}

// CopyOptions configures an image copy operation.
type CopyOptions struct {
	// Source credentials for authentication.
	SrcCreds *Credentials
	// Destination credentials for authentication.
	DestCreds *Credentials

	// SrcTLSVerify enables TLS verification for source (default: true).
	SrcTLSVerify *bool
	// DestTLSVerify enables TLS verification for destination (default: true).
	DestTLSVerify *bool

	// PreserveDigests preserves digests during copy.
	PreserveDigests bool
	// All copies all images if source is a multi-architecture image.
	All bool

	// AdditionalTags specifies additional tags to add to the destination image.
	AdditionalTags []string

	// Format specifies the manifest format (oci, v2s1, v2s2).
	Format string

	// Quiet suppresses output.
	Quiet bool
}

// InspectOptions configures an image inspect operation.
type InspectOptions struct {
	// Creds for authentication.
	Creds *Credentials
	// TLSVerify enables TLS verification (default: true).
	TLSVerify *bool
}

// ImageInfo represents the result of an image inspection.
type ImageInfo struct {
	Name          string            `json:"Name"`
	Digest        string            `json:"Digest"`
	RepoTags      []string          `json:"RepoTags"`
	Created       string            `json:"Created"`
	DockerVersion string            `json:"DockerVersion"`
	Labels        map[string]string `json:"Labels"`
	Architecture  string            `json:"Architecture"`
	Os            string            `json:"Os"`
	Layers        []string          `json:"Layers"`
	LayersData    []LayerData       `json:"LayersData"`
	Env           []string          `json:"Env"`
}

// LayerData contains information about an image layer.
type LayerData struct {
	MIMEType    string `json:"MIMEType"`
	Digest      string `json:"Digest"`
	Size        int64  `json:"Size"`
	Annotations any    `json:"Annotations"`
}

// Skopeo provides operations for copying and managing container images.
type Skopeo struct {
	config SkopeoConfig
}

// NewSkopeo creates a new Skopeo instance with the given configuration.
func NewSkopeo(config SkopeoConfig) *Skopeo {
	if config.SkopeoPath == "" {
		config.SkopeoPath = "skopeo"
	}
	return &Skopeo{config: config}
}

// ImageRef constructs a full image reference with the given transport.
func ImageRef(transport Transport, image string) string {
	return string(transport) + image
}

// Copy copies an image from source to destination.
func (s *Skopeo) Copy(ctx context.Context, src, dest string, opts *CopyOptions) error {
	args := []string{"copy"}

	if opts != nil {
		if opts.SrcCreds != nil {
			args = append(args, "--src-creds", formatCreds(opts.SrcCreds))
		}
		if opts.DestCreds != nil {
			args = append(args, "--dest-creds", formatCreds(opts.DestCreds))
		}
		if opts.SrcTLSVerify != nil {
			args = append(args, fmt.Sprintf("--src-tls-verify=%t", *opts.SrcTLSVerify))
		}
		if opts.DestTLSVerify != nil {
			args = append(args, fmt.Sprintf("--dest-tls-verify=%t", *opts.DestTLSVerify))
		}
		if opts.PreserveDigests {
			args = append(args, "--preserve-digests")
		}
		if opts.All {
			args = append(args, "--all")
		}
		for _, tag := range opts.AdditionalTags {
			args = append(args, "--additional-tag", tag)
		}
		if opts.Format != "" {
			args = append(args, "--format", opts.Format)
		}
		if opts.Quiet {
			args = append(args, "--quiet")
		}
	}

	if s.config.InsecurePolicy {
		args = append(args, "--insecure-policy")
	}

	args = append(args, src, dest)

	return s.run(ctx, args)
}

// Inspect retrieves information about an image.
func (s *Skopeo) Inspect(ctx context.Context, image string, opts *InspectOptions) (*ImageInfo, error) {
	args := []string{"inspect"}

	if opts != nil {
		if opts.Creds != nil {
			args = append(args, "--creds", formatCreds(opts.Creds))
		}
		if opts.TLSVerify != nil {
			args = append(args, fmt.Sprintf("--tls-verify=%t", *opts.TLSVerify))
		}
	}

	if s.config.InsecurePolicy {
		args = append(args, "--insecure-policy")
	}

	// If image doesn't have a transport prefix, assume docker://
	if !hasTransportPrefix(image) {
		image = ImageRef(TransportDocker, image)
	}
	args = append(args, image)

	output, err := s.runOutput(ctx, args)
	if err != nil {
		return nil, err
	}

	var info ImageInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	return &info, nil
}

// Version returns the skopeo version.
func (s *Skopeo) Version(ctx context.Context) (string, error) {
	output, err := s.runOutput(ctx, []string{"--version"})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// run executes a skopeo command.
func (s *Skopeo) run(ctx context.Context, args []string) error {
	slog.Debug("executing skopeo command", "command", args[0], "args_count", len(args))

	cmd := exec.CommandContext(ctx, s.config.SkopeoPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			slog.Error("skopeo command failed", "command", args[0], "stderr", errMsg)
			return fmt.Errorf("skopeo %s failed: %s", args[0], errMsg)
		}
		slog.Error("skopeo command failed", "command", args[0], "error", err)
		return fmt.Errorf("skopeo %s failed: %w", args[0], err)
	}

	slog.Debug("skopeo command succeeded", "command", args[0])
	return nil
}

// runOutput executes a skopeo command and returns the output.
func (s *Skopeo) runOutput(ctx context.Context, args []string) ([]byte, error) {
	slog.Debug("executing skopeo command", "command", args[0], "args_count", len(args))

	cmd := exec.CommandContext(ctx, s.config.SkopeoPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			slog.Error("skopeo command failed", "command", args[0], "stderr", errMsg)
			return nil, fmt.Errorf("skopeo %s failed: %s", args[0], errMsg)
		}
		slog.Error("skopeo command failed", "command", args[0], "error", err)
		return nil, fmt.Errorf("skopeo %s failed: %w", args[0], err)
	}

	slog.Debug("skopeo command succeeded", "command", args[0], "output_size", stdout.Len())
	return stdout.Bytes(), nil
}

// formatCreds formats credentials as username:password.
func formatCreds(creds *Credentials) string {
	if creds.Password == "" {
		return creds.Username
	}
	return creds.Username + ":" + creds.Password
}

// hasTransportPrefix checks if the image reference already has a transport prefix.
func hasTransportPrefix(image string) bool {
	return strings.HasPrefix(image, string(TransportDocker)) ||
		strings.HasPrefix(image, string(TransportDockerDaemon))
}

// Bool is a helper to create a pointer to a bool value.
func Bool(v bool) *bool {
	return &v
}

// ExtractImageName extracts the image name from a full reference.
// e.g., "ghcr.io/org/app:v1" -> "org/app:v1"
// e.g., "alpine:latest" -> "alpine:latest"
func ExtractImageName(ref string) string {
	// Remove transport prefix if present
	ref = strings.TrimPrefix(ref, string(TransportDocker))
	ref = strings.TrimPrefix(ref, string(TransportDockerDaemon))

	// Check if it has a registry prefix (contains a dot before the first slash)
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 && strings.Contains(parts[0], ".") {
		// Has registry prefix, return the rest
		return parts[1]
	}

	// Check for docker.io/library/ prefix
	if strings.HasPrefix(ref, "docker.io/library/") {
		return strings.TrimPrefix(ref, "docker.io/library/")
	}
	if strings.HasPrefix(ref, "docker.io/") {
		return strings.TrimPrefix(ref, "docker.io/")
	}

	return ref
}

// DetermineSourceRef determines the appropriate skopeo source reference.
// It checks if the image exists in the local Docker daemon first.
func DetermineSourceRef(source string) string {
	// If it already has a transport prefix, use as-is
	if strings.HasPrefix(source, string(TransportDocker)) ||
		strings.HasPrefix(source, string(TransportDockerDaemon)) ||
		strings.HasPrefix(source, "oci:") ||
		strings.HasPrefix(source, "dir:") {
		return source
	}

	// Check if image exists locally in Docker daemon
	docker, err := NewDockerClient()
	if err == nil {
		ctx := context.Background()
		exists, checkErr := docker.ImageExists(ctx, source)
		if checkErr == nil && exists {
			// Image found in local Docker daemon
			return ImageRef(TransportDockerDaemon, source)
		}
	}

	// Not found locally - determine the remote registry reference
	if !strings.Contains(source, "/") {
		// Bare image name like "alpine" - assume Docker Hub library
		return ImageRef(TransportDocker, "docker.io/library/"+source)
	}

	// Check if it looks like a Docker Hub image (no dots in first segment)
	parts := strings.SplitN(source, "/", 2)
	if !strings.Contains(parts[0], ".") && !strings.Contains(parts[0], ":") {
		// Looks like docker.io user/repo format
		return ImageRef(TransportDocker, "docker.io/"+source)
	}

	return ImageRef(TransportDocker, source)
}
