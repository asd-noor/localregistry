package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tagsCmd = &cobra.Command{
	Use:   "tags <repository>",
	Short: "List tags for a repository",
	Long:  "Retrieve and display all tags for the specified repository.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]

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
