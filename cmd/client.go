package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"localregistry/internal/api"
)

var (
	registryHost string
	registryPort int
	username     string
	password     string
	insecure     bool
	timeout      time.Duration
)

func newClient() (*api.Client, error) {
	address := fmt.Sprintf("%s:%d", registryHost, registryPort)
	cfg := api.ClientConfig{
		Address:  address,
		Username: username,
		Password: password,
		Insecure: insecure,
		Timeout:  timeout,
	}

	slog.Debug("creating api client", "address", address, "insecure", insecure, "timeout", timeout)

	client, err := api.NewClient(cfg)
	if err != nil {
		slog.Error("failed to create api client", "error", err)
		return nil, err
	}

	slog.Debug("api client created successfully")
	return client, nil
}

func newContext() context.Context {
	return context.Background()
}

func exitOnError(err error) {
	if err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

// parseImageRef parses an image reference into name and reference (tag or digest) parts.
// Format: [repository/]name[:tag] or [repository/]name[@digest]
// Returns (name, reference) where reference defaults to "latest" if not specified.
//
// Examples:
//   - "alpine" -> ("alpine", "latest")
//   - "alpine:3.18" -> ("alpine", "3.18")
//   - "library/alpine:latest" -> ("library/alpine", "latest")
//   - "alpine@sha256:abc123" -> ("alpine", "sha256:abc123")
func parseImageRef(ref string) (name, reference string) {
	// Check for digest reference first (@sha256:...)
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		return ref[:idx], ref[idx+1:]
	}

	// Find last colon for tag - but be careful not to split on port numbers
	// A tag comes after the last colon that's not followed by a slash
	idx := strings.LastIndex(ref, ":")
	if idx == -1 {
		// No tag specified
		return ref, "latest"
	}

	// Check if the colon is part of a port (e.g., localhost:5000/image)
	// If there's a slash after the colon, it's a port, not a tag
	if strings.Contains(ref[idx:], "/") {
		return ref, "latest"
	}

	return ref[:idx], ref[idx+1:]
}

// stripTag removes the tag or digest portion from an image reference.
// Returns just the image name (with repository path if present).
func stripTag(ref string) string {
	name, _ := parseImageRef(ref)
	return name
}
