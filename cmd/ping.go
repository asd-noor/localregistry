package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Check registry connectivity",
	Long:  "Verify that the registry is reachable and implements Docker Registry API V2.",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := newClient()
		exitOnError(err)

		err = client.Ping(newContext())
		exitOnError(err)

		fmt.Println("Registry is reachable and implements V2 API")
	},
}

func init() {
	rootCmd.AddCommand(pingCmd)
}
