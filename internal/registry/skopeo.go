package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	// TransportDockerArchive is the docker-archive: transport for docker save format.
	TransportDockerArchive Transport = "docker-archive:"
	// TransportOCI is the oci: transport for OCI layout directories.
	TransportOCI Transport = "oci:"
	// TransportDir is the dir: transport for directory storage.
	TransportDir Transport = "dir:"
	// TransportContainersStorage is the containers-storage: transport.
	TransportContainersStorage Transport = "containers-storage:"
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
	// Raw returns raw manifest instead of parsed output.
	Raw bool
	// Config returns image configuration instead of manifest.
	Config bool
}

// DeleteOptions configures an image delete operation.
type DeleteOptions struct {
	// Creds for authentication.
	Creds *Credentials
	// TLSVerify enables TLS verification (default: true).
	TLSVerify *bool
}

// SyncOptions configures a registry sync operation.
type SyncOptions struct {
	// SrcCreds for source authentication.
	SrcCreds *Credentials
	// DestCreds for destination authentication.
	DestCreds *Credentials

	// SrcTLSVerify enables TLS verification for source.
	SrcTLSVerify *bool
	// DestTLSVerify enables TLS verification for destination.
	DestTLSVerify *bool

	// All syncs all images including multi-arch.
	All bool

	// DryRun shows what would be synced without syncing.
	DryRun bool
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

// Pull copies an image from a remote registry to the local Docker daemon.
// This is a convenience wrapper around Copy with docker-daemon: destination.
func (s *Skopeo) Pull(ctx context.Context, image string, opts *CopyOptions) error {
	src := ImageRef(TransportDocker, image)
	// For docker-daemon, the reference format is docker-daemon:image:tag
	dest := ImageRef(TransportDockerDaemon, image)
	return s.Copy(ctx, src, dest, opts)
}

// Push copies an image from the local Docker daemon to a remote registry.
// This is a convenience wrapper around Copy with docker-daemon: source.
func (s *Skopeo) Push(ctx context.Context, localImage, remoteImage string, opts *CopyOptions) error {
	src := ImageRef(TransportDockerDaemon, localImage)
	dest := ImageRef(TransportDocker, remoteImage)
	return s.Copy(ctx, src, dest, opts)
}

// CopyToDir copies an image to a local directory.
func (s *Skopeo) CopyToDir(ctx context.Context, image, dir string, opts *CopyOptions) error {
	src := ImageRef(TransportDocker, image)
	dest := ImageRef(TransportDir, dir)
	return s.Copy(ctx, src, dest, opts)
}

// CopyToOCI copies an image to a local OCI layout directory.
func (s *Skopeo) CopyToOCI(ctx context.Context, image, path, tag string, opts *CopyOptions) error {
	src := ImageRef(TransportDocker, image)
	dest := fmt.Sprintf("%s%s:%s", TransportOCI, path, tag)
	return s.Copy(ctx, src, dest, opts)
}

// CopyFromArchive copies an image from a docker-archive file to a registry.
func (s *Skopeo) CopyFromArchive(ctx context.Context, archivePath, destImage string, opts *CopyOptions) error {
	src := ImageRef(TransportDockerArchive, archivePath)
	dest := ImageRef(TransportDocker, destImage)
	return s.Copy(ctx, src, dest, opts)
}

// CopyToArchive copies an image to a docker-archive file.
func (s *Skopeo) CopyToArchive(ctx context.Context, image, archivePath string, opts *CopyOptions) error {
	src := ImageRef(TransportDocker, image)
	dest := ImageRef(TransportDockerArchive, archivePath)
	return s.Copy(ctx, src, dest, opts)
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
		if opts.Raw {
			args = append(args, "--raw")
		}
		if opts.Config {
			args = append(args, "--config")
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

	// If raw output requested, we can't parse it as ImageInfo
	if opts != nil && opts.Raw {
		return &ImageInfo{Name: image}, nil
	}

	var info ImageInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	return &info, nil
}

// InspectRaw retrieves the raw manifest of an image.
func (s *Skopeo) InspectRaw(ctx context.Context, image string, opts *InspectOptions) ([]byte, error) {
	if opts == nil {
		opts = &InspectOptions{}
	}
	opts.Raw = true

	args := []string{"inspect", "--raw"}

	if opts.Creds != nil {
		args = append(args, "--creds", formatCreds(opts.Creds))
	}
	if opts.TLSVerify != nil {
		args = append(args, fmt.Sprintf("--tls-verify=%t", *opts.TLSVerify))
	}

	if s.config.InsecurePolicy {
		args = append(args, "--insecure-policy")
	}

	if !hasTransportPrefix(image) {
		image = ImageRef(TransportDocker, image)
	}
	args = append(args, image)

	return s.runOutput(ctx, args)
}

// Delete removes an image from a registry.
func (s *Skopeo) Delete(ctx context.Context, image string, opts *DeleteOptions) error {
	args := []string{"delete"}

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

	if !hasTransportPrefix(image) {
		image = ImageRef(TransportDocker, image)
	}
	args = append(args, image)

	return s.run(ctx, args)
}

// ListTags returns a list of tags for the given repository.
func (s *Skopeo) ListTags(ctx context.Context, repo string, creds *Credentials, tlsVerify *bool) ([]string, error) {
	args := []string{"list-tags"}

	if creds != nil {
		args = append(args, "--creds", formatCreds(creds))
	}
	if tlsVerify != nil {
		args = append(args, fmt.Sprintf("--tls-verify=%t", *tlsVerify))
	}

	if s.config.InsecurePolicy {
		args = append(args, "--insecure-policy")
	}

	if !hasTransportPrefix(repo) {
		repo = ImageRef(TransportDocker, repo)
	}
	args = append(args, repo)

	output, err := s.runOutput(ctx, args)
	if err != nil {
		return nil, err
	}

	var result struct {
		Repository string   `json:"Repository"`
		Tags       []string `json:"Tags"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse list-tags output: %w", err)
	}

	return result.Tags, nil
}

// Sync synchronizes images between a source and destination.
func (s *Skopeo) Sync(ctx context.Context, srcType, src, destType, dest string, opts *SyncOptions) error {
	args := []string{"sync", "--src", srcType, "--dest", destType}

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
		if opts.All {
			args = append(args, "--all")
		}
		if opts.DryRun {
			args = append(args, "--dry-run")
		}
	}

	if s.config.InsecurePolicy {
		args = append(args, "--insecure-policy")
	}

	args = append(args, src, dest)

	return s.run(ctx, args)
}

// SyncFromRegistry syncs images from a registry to a local directory.
func (s *Skopeo) SyncFromRegistry(ctx context.Context, registry, destDir string, opts *SyncOptions) error {
	return s.Sync(ctx, "docker", registry, "dir", destDir, opts)
}

// SyncToRegistry syncs images from a local directory to a registry.
func (s *Skopeo) SyncToRegistry(ctx context.Context, srcDir, registry string, opts *SyncOptions) error {
	return s.Sync(ctx, "dir", srcDir, "docker", registry, opts)
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
	cmd := exec.CommandContext(ctx, s.config.SkopeoPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			return fmt.Errorf("skopeo %s failed: %s", args[0], errMsg)
		}
		return fmt.Errorf("skopeo %s failed: %w", args[0], err)
	}

	return nil
}

// runOutput executes a skopeo command and returns the output.
func (s *Skopeo) runOutput(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, s.config.SkopeoPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			return nil, fmt.Errorf("skopeo %s failed: %s", args[0], errMsg)
		}
		return nil, fmt.Errorf("skopeo %s failed: %w", args[0], err)
	}

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
	transports := []Transport{
		TransportDocker,
		TransportDockerDaemon,
		TransportDockerArchive,
		TransportOCI,
		TransportDir,
		TransportContainersStorage,
	}
	for _, t := range transports {
		if strings.HasPrefix(image, string(t)) {
			return true
		}
	}
	return false
}

// Bool is a helper to create a pointer to a bool value.
func Bool(v bool) *bool {
	return &v
}
