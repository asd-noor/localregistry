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
	Use:   "get <repository> <reference>",
	Short: "Get a manifest",
	Long: `Retrieve a manifest by repository name and reference (tag or digest).

Examples:
  localregistry manifest get myrepo latest
  localregistry manifest get myrepo sha256:abc123...`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]
		reference := args[1]

		client, err := newClient()
		exitOnError(err)

		manifest, err := client.GetManifest(newContext(), repo, reference)
		exitOnError(err)

		fmt.Fprintf(os.Stderr, "Content-Type: %s\n", manifest.ContentType)
		fmt.Fprintf(os.Stderr, "Digest: %s\n\n", manifest.Digest)
		fmt.Println(string(manifest.Body))
	},
}

var manifestHeadCmd = &cobra.Command{
	Use:   "head <repository> <reference>",
	Short: "Check if a manifest exists",
	Long: `Check if a manifest exists and display its metadata without fetching the body.

Examples:
  localregistry manifest head myrepo latest
  localregistry manifest head myrepo sha256:abc123...`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]
		reference := args[1]

		client, err := newClient()
		exitOnError(err)

		manifest, err := client.HeadManifest(newContext(), repo, reference)
		exitOnError(err)

		fmt.Printf("Content-Type: %s\n", manifest.ContentType)
		fmt.Printf("Digest: %s\n", manifest.Digest)
	},
}

var manifestDeleteCmd = &cobra.Command{
	Use:   "delete <repository> <reference>",
	Short: "Delete a manifest",
	Long: `Delete a manifest by repository name and reference (tag or digest).

When deleting by tag, the manifest digest is first retrieved, then deleted.

Examples:
  localregistry manifest delete myrepo latest
  localregistry manifest delete myrepo sha256:abc123...`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]
		reference := args[1]

		client, err := newClient()
		exitOnError(err)

		ctx := newContext()

		if isDigest(reference) {
			err = client.DeleteManifest(ctx, repo, reference)
		} else {
			err = client.DeleteManifestByTag(ctx, repo, reference)
		}
		exitOnError(err)

		fmt.Printf("Manifest '%s@%s' deleted successfully\n", repo, reference)
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
