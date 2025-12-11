package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tagsCmd = &cobra.Command{
	Use:   "tags <image>",
	Short: "List tags for a repository",
	Long: `Retrieve and display all tags for the specified image/repository.

If a tag is provided in the image reference, it will be ignored.

Examples:
  local-registry tags alpine
  local-registry tags library/alpine
  local-registry tags alpine:latest  # tag is ignored, lists all tags`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Strip any tag from the input - we only need the repository name
		repo := stripTag(args[0])

		client, err := newClient()
		exitOnError(err)

		tags, err := client.ListAllTags(newContext(), repo)
		exitOnError(err)

		if len(tags) == 0 {
			fmt.Printf("No tags found for repository '%s'\n", repo)
			return
		}

		for _, tag := range tags {
			fmt.Println(tag)
		}
	},
}

func init() {
	rootCmd.AddCommand(tagsCmd)
}
