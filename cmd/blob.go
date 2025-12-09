package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	blobOutput string
)

var blobCmd = &cobra.Command{
	Use:   "blob",
	Short: "Manage blobs",
	Long:  "Commands for retrieving, inspecting, and deleting blobs.",
}

var blobGetCmd = &cobra.Command{
	Use:   "get <repository> <digest>",
	Short: "Get a blob",
	Long: `Retrieve a blob by repository name and digest.

The blob content is written to stdout by default, or to a file if -o is specified.

Examples:
  localregistry blob get myrepo sha256:abc123...
  localregistry blob get myrepo sha256:abc123... -o layer.tar.gz`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]
		digest := args[1]

		client, err := newClient()
		exitOnError(err)

		reader, info, err := client.GetBlob(newContext(), repo, digest)
		exitOnError(err)
		defer reader.Close()

		var out io.Writer = os.Stdout
		if blobOutput != "" {
			f, err := os.Create(blobOutput)
			exitOnError(err)
			defer f.Close()
			out = f
		}

		written, err := io.Copy(out, reader)
		exitOnError(err)

		if blobOutput != "" {
			fmt.Fprintf(os.Stderr, "Digest: %s\n", info.Digest)
			fmt.Fprintf(os.Stderr, "Size: %d bytes\n", info.ContentLength)
			fmt.Fprintf(os.Stderr, "Written: %d bytes to %s\n", written, blobOutput)
		}
	},
}

var blobHeadCmd = &cobra.Command{
	Use:   "head <repository> <digest>",
	Short: "Check if a blob exists",
	Long: `Check if a blob exists and display its metadata.

Examples:
  localregistry blob head myrepo sha256:abc123...`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]
		digest := args[1]

		client, err := newClient()
		exitOnError(err)

		info, err := client.HeadBlob(newContext(), repo, digest)
		exitOnError(err)

		fmt.Printf("Digest: %s\n", info.Digest)
		fmt.Printf("Size: %d bytes\n", info.ContentLength)
	},
}

var blobDeleteCmd = &cobra.Command{
	Use:   "delete <repository> <digest>",
	Short: "Delete a blob",
	Long: `Delete a blob by repository name and digest.

Examples:
  localregistry blob delete myrepo sha256:abc123...`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]
		digest := args[1]

		client, err := newClient()
		exitOnError(err)

		err = client.DeleteBlob(newContext(), repo, digest)
		exitOnError(err)

		fmt.Printf("Blob '%s' deleted from '%s'\n", digest, repo)
	},
}

func init() {
	blobGetCmd.Flags().StringVarP(&blobOutput, "output", "o", "", "Output file (default: stdout)")

	blobCmd.AddCommand(blobGetCmd)
	blobCmd.AddCommand(blobHeadCmd)
	blobCmd.AddCommand(blobDeleteCmd)
	rootCmd.AddCommand(blobCmd)
}
