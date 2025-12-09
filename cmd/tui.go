package cmd

import (
	"localregistry/internal/api"
	"localregistry/internal/tui"

	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI",
	Long:  "Start the interactive terminal user interface for browsing and managing the registry.",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := api.ClientConfig{
			Address:  registryURL,
			Username: username,
			Password: password,
			Insecure: insecure,
			Timeout:  timeout,
		}

		client, err := api.NewClient(cfg)
		exitOnError(err)

		err = tui.Run(client)
		exitOnError(err)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
