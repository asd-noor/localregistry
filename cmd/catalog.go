package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var catalogCmd = &cobra.Command{
	Use:     "catalog",
	Aliases: []string{"repos", "repositories"},
	Short:   "List repositories in the registry",
	Long:    "Retrieve and display all repositories available in the registry.",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := newClient()
		exitOnError(err)

		repos, err := client.ListAllRepositories(newContext())
		exitOnError(err)

		if len(repos) == 0 {
			fmt.Println("No repositories found")
			return
		}

		for _, repo := range repos {
			fmt.Println(repo)
		}
	},
}

func init() {
	rootCmd.AddCommand(catalogCmd)
}
