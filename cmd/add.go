package cmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"localregistry/internal/registry"

	"github.com/spf13/cobra"
)

var (
	addAll        bool
	addQuiet      bool
	addSrcCreds   string
	addDestCreds  string
	addSrcVerify  bool
	addDestVerify bool
)

var addCmd = &cobra.Command{
	Use:   "add <source-image> [target]",
	Short: "Add an image to the local registry",
	Long: `Mirror an image into the local registry using skopeo.

The source image can be from any registry (docker.io, ghcr.io, etc.) or
from the local Docker daemon. If target is not specified, the image name
(without registry prefix) is used.

Examples:
  # Add alpine from Docker Hub
  localregistry add alpine

  # Add with custom target name
  localregistry add ghcr.io/myorg/app:v1.0 myapp:v1.0

  # Add from local Docker daemon (auto-detected)
  localregistry add mylocal:latest

  # Add multi-architecture image
  localregistry add --all alpine:latest`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		source := args[0]
		if strings.TrimSpace(source) == "" {
			fmt.Fprintln(os.Stderr, "Error: source image cannot be empty")
			os.Exit(1)
		}

		target := ""
		if len(args) > 1 {
			target = args[1]
		} else {
			// Extract image name from source (strip registry prefix if present)
			target = extractImageName(source)
		}

		// Build destination reference
		dest := fmt.Sprintf("%s:%d/%s", registryHost, registryPort, target)

		skopeo := registry.NewSkopeo(registry.SkopeoConfig{
			InsecurePolicy: insecure,
		})

		// Determine source transport
		srcRef := determineSourceRef(source)

		// Build copy options
		opts := &registry.CopyOptions{
			All:   addAll,
			Quiet: addQuiet,
		}

		// Handle TLS verification
		// Source TLS verify defaults true for remote registries
		// Destination TLS verify defaults false for local HTTP registry
		opts.SrcTLSVerify = &addSrcVerify
		opts.DestTLSVerify = &addDestVerify

		// Handle credentials
		if addSrcCreds != "" {
			parts := strings.SplitN(addSrcCreds, ":", 2)
			opts.SrcCreds = &registry.Credentials{Username: parts[0]}
			if len(parts) > 1 {
				opts.SrcCreds.Password = parts[1]
			}
		}
		if addDestCreds != "" {
			parts := strings.SplitN(addDestCreds, ":", 2)
			opts.DestCreds = &registry.Credentials{Username: parts[0]}
			if len(parts) > 1 {
				opts.DestCreds.Password = parts[1]
			}
		} else if username != "" {
			opts.DestCreds = &registry.Credentials{
				Username: username,
				Password: password,
			}
		}

		destRef := registry.ImageRef(registry.TransportDocker, dest)

		if !addQuiet {
			fmt.Printf("Copying %s -> %s\n", source, dest)
		}

		ctx := context.Background()
		if err := skopeo.Copy(ctx, srcRef, destRef, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if !addQuiet {
			fmt.Printf("Successfully added %s\n", dest)
		}
	},
}

// extractImageName extracts the image name from a full reference.
// e.g., "ghcr.io/org/app:v1" -> "org/app:v1"
// e.g., "alpine:latest" -> "alpine:latest"
func extractImageName(ref string) string {
	// Remove transport prefix if present
	ref = strings.TrimPrefix(ref, "docker://")
	ref = strings.TrimPrefix(ref, "docker-daemon:")

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

// determineSourceRef determines the appropriate skopeo source reference.
// It checks if the image exists in the local Docker daemon first.
func determineSourceRef(source string) string {
	// If it already has a transport prefix, use as-is
	if strings.HasPrefix(source, "docker://") ||
		strings.HasPrefix(source, "docker-daemon:") ||
		strings.HasPrefix(source, "oci:") ||
		strings.HasPrefix(source, "dir:") {
		return source
	}

	// Check if image exists locally in Docker daemon
	docker, err := registry.NewDockerClient()
	if err == nil {
		ctx := context.Background()
		exists, checkErr := docker.ImageExists(ctx, source)
		if checkErr == nil && exists {
			// Image found in local Docker daemon
			fmt.Printf("Found local image: %s\n", source)
			return registry.ImageRef(registry.TransportDockerDaemon, source)
		}
	}

	// Not found locally - determine the remote registry reference
	if !strings.Contains(source, "/") {
		// Bare image name like "alpine" - assume Docker Hub library
		return registry.ImageRef(registry.TransportDocker, "docker.io/library/"+source)
	}

	// Check if it looks like a Docker Hub image (no dots in first segment)
	parts := strings.SplitN(source, "/", 2)
	if !strings.Contains(parts[0], ".") && !strings.Contains(parts[0], ":") {
		// Looks like docker.io user/repo format
		return registry.ImageRef(registry.TransportDocker, "docker.io/"+source)
	}

	return registry.ImageRef(registry.TransportDocker, source)
}

var inspectCmd = &cobra.Command{
	Use:   "inspect <image>",
	Short: "Inspect an image in the registry",
	Long: `Retrieve and display information about an image in the registry.

Examples:
  localregistry inspect myapp:latest
  localregistry inspect alpine`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		image := args[0]

		// Build full image reference
		fullRef := fmt.Sprintf("%s:%d/%s", registryHost, registryPort, image)

		skopeo := registry.NewSkopeo(registry.SkopeoConfig{
			InsecurePolicy: insecure,
		})

		tlsVerify := !insecure
		opts := &registry.InspectOptions{
			TLSVerify: &tlsVerify,
		}

		if username != "" {
			opts.Creds = &registry.Credentials{
				Username: username,
				Password: password,
			}
		}

		ctx := context.Background()
		info, err := skopeo.Inspect(ctx, fullRef, opts)
		exitOnError(err)

		fmt.Printf("Name:         %s\n", info.Name)
		fmt.Printf("Digest:       %s\n", info.Digest)
		fmt.Printf("Created:      %s\n", info.Created)
		fmt.Printf("Architecture: %s\n", info.Architecture)
		fmt.Printf("OS:           %s\n", info.Os)

		if len(info.RepoTags) > 0 {
			fmt.Println("Tags:")
			for _, tag := range info.RepoTags {
				fmt.Printf("  - %s\n", tag)
			}
		}

		if len(info.Layers) > 0 {
			fmt.Printf("Layers:       %d\n", len(info.Layers))
			for i, layer := range info.Layers {
				// Truncate layer digest for display
				short := layer
				if len(layer) > 20 {
					short = layer[:20] + "..."
				}
				fmt.Printf("  %d: %s\n", i+1, short)
			}
		}
	},
}

var copyCmd = &cobra.Command{
	Use:   "copy <source> <destination>",
	Short: "Copy an image between registries",
	Long: `Copy an image from one location to another using skopeo.

Supports various transports:
  - docker://registry/image:tag (remote registry)
  - docker-daemon:image:tag (local Docker daemon)
  - oci:path:tag (OCI layout directory)
  - dir:path (directory)

Examples:
  # Copy from Docker Hub to local registry
  localregistry copy docker://alpine docker://localhost:5000/alpine

  # Copy from local registry to another registry
  localregistry copy docker://localhost:5000/myapp docker://ghcr.io/myorg/myapp`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		source := args[0]
		dest := args[1]

		skopeo := registry.NewSkopeo(registry.SkopeoConfig{
			InsecurePolicy: insecure,
		})

		opts := &registry.CopyOptions{
			All:   addAll,
			Quiet: addQuiet,
		}

		srcVerify := addSrcVerify
		destVerify := !insecure
		opts.SrcTLSVerify = &srcVerify
		opts.DestTLSVerify = &destVerify

		if !addQuiet {
			fmt.Printf("Copying %s -> %s\n", source, dest)
		}

		ctx := context.Background()
		if err := skopeo.Copy(ctx, source, dest, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if !addQuiet {
			fmt.Printf("Successfully copied to %s\n", path.Base(dest))
		}
	},
}

func init() {
	// Add command flags
	addCmd.Flags().BoolVarP(&addAll, "all", "a", false, "Copy all architectures (multi-arch images)")
	addCmd.Flags().BoolVarP(&addQuiet, "quiet", "q", false, "Suppress output")
	addCmd.Flags().StringVar(&addSrcCreds, "src-creds", "", "Source credentials (user:password)")
	addCmd.Flags().StringVar(&addDestCreds, "dest-creds", "", "Destination credentials (user:password)")
	addCmd.Flags().BoolVar(&addSrcVerify, "src-tls-verify", true, "Verify source TLS certificates")
	addCmd.Flags().BoolVar(&addDestVerify, "dest-tls-verify", false, "Verify destination TLS certificates (set true for HTTPS registries)")

	// Copy command flags (reuse add flags)
	copyCmd.Flags().BoolVarP(&addAll, "all", "a", false, "Copy all architectures (multi-arch images)")
	copyCmd.Flags().BoolVarP(&addQuiet, "quiet", "q", false, "Suppress output")
	copyCmd.Flags().StringVar(&addSrcCreds, "src-creds", "", "Source credentials (user:password)")
	copyCmd.Flags().StringVar(&addDestCreds, "dest-creds", "", "Destination credentials (user:password)")
	copyCmd.Flags().BoolVar(&addSrcVerify, "src-tls-verify", true, "Verify source TLS certificates")
	copyCmd.Flags().BoolVar(&addDestVerify, "dest-tls-verify", false, "Verify destination TLS certificates")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(copyCmd)
}
