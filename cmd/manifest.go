package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var manifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Manage manifests",
	Long:  "Commands for retrieving, inspecting, and deleting image manifests.",
}

var manifestGetCmd = &cobra.Command{
	Use:   "get <image>",
	Short: "Get a manifest",
	Long: `Retrieve a manifest by image reference.

The image reference can include a tag or digest:
  - name:tag (e.g., alpine:latest)
  - name@sha256:... (e.g., alpine@sha256:abc123...)
  - repository/name:tag (e.g., library/alpine:latest)

If no tag is specified, "latest" is assumed.

Examples:
  local-registry manifest get alpine
  local-registry manifest get alpine:3.18
  local-registry manifest get alpine@sha256:abc123...
  local-registry manifest get library/alpine:latest`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, reference := parseImageRef(args[0])

		client, err := newClient()
		exitOnError(err)

		manifest, err := client.GetManifest(newContext(), name, reference)
		exitOnError(err)

		fmt.Fprintf(os.Stderr, "Content-Type: %s\n", manifest.ContentType)
		fmt.Fprintf(os.Stderr, "Digest: %s\n\n", manifest.Digest)
		fmt.Println(string(manifest.Body))
	},
}

var manifestHeadCmd = &cobra.Command{
	Use:   "head <image>",
	Short: "Check if a manifest exists",
	Long: `Check if a manifest exists and display its metadata without fetching the body.

The image reference can include a tag or digest:
  - name:tag (e.g., alpine:latest)
  - name@sha256:... (e.g., alpine@sha256:abc123...)

If no tag is specified, "latest" is assumed.

Examples:
  local-registry manifest head alpine
  local-registry manifest head alpine:3.18
  local-registry manifest head alpine@sha256:abc123...`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, reference := parseImageRef(args[0])

		client, err := newClient()
		exitOnError(err)

		manifest, err := client.HeadManifest(newContext(), name, reference)
		exitOnError(err)

		fmt.Printf("Content-Type: %s\n", manifest.ContentType)
		fmt.Printf("Digest: %s\n", manifest.Digest)
	},
}

var manifestDeleteCmd = &cobra.Command{
	Use:   "delete <image>",
	Short: "Delete a manifest",
	Long: `Delete a manifest by image reference.

The image reference can include a tag or digest:
  - name:tag (e.g., alpine:latest)
  - name@sha256:... (e.g., alpine@sha256:abc123...)

When deleting by tag, the manifest digest is first retrieved, then deleted.

Examples:
  local-registry manifest delete alpine:latest
  local-registry manifest delete alpine@sha256:abc123...`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, reference := parseImageRef(args[0])

		client, err := newClient()
		exitOnError(err)

		ctx := newContext()

		if isDigest(reference) {
			err = client.DeleteManifest(ctx, name, reference)
		} else {
			err = client.DeleteManifestByTag(ctx, name, reference)
		}
		exitOnError(err)

		fmt.Printf("Manifest '%s:%s' deleted successfully\n", name, reference)
	},
}

func isDigest(ref string) bool {
	return len(ref) > 7 && ref[:7] == "sha256:"
}

func init() {
	manifestCmd.AddCommand(manifestGetCmd)
	manifestCmd.AddCommand(manifestHeadCmd)
	manifestCmd.AddCommand(manifestDeleteCmd)
	rootCmd.AddCommand(manifestCmd)
}
