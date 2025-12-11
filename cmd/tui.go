package cmd

import (
	"fmt"

	"local-registry/internal/api"
	"local-registry/internal/registry"
	"local-registry/internal/tui"

	"github.com/spf13/cobra"
)

var tuiContainerName string

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI",
	Long: `Start the interactive terminal user interface for browsing and managing the registry.

The TUI provides:
  - Browse repositories and tags
  - View image details (layers, sizes)
  - Add images to the registry
  - Delete tags
  - View container logs
  - Server management (start/stop/restart)
  - Garbage collection

Examples:
  local-registry tui
  local-registry tui --container my-registry`,
	Run: func(cmd *cobra.Command, args []string) {
		address := fmt.Sprintf("%s:%d", registryHost, registryPort)
		cfg := api.ClientConfig{
			Address:  address,
			Username: username,
			Password: password,
			Insecure: insecure,
			Timeout:  timeout,
		}

		client, err := api.NewClient(cfg)
		exitOnError(err)

		// Use config value if flag wasn't explicitly set
		cname := tuiContainerName
		if !cmd.Flags().Changed("container") && GetConfig() != nil {
			cname = GetConfig().Registry.ContainerName
		}

		// Get data directory from config
		dataDir := ""
		if GetConfig() != nil {
			dataDir = GetConfig().Registry.DataDir
		}

		// Try to set up Docker client and registry manager for server features
		docker, dockerErr := registry.NewDockerClient()
		var reg *registry.Registry
		if dockerErr == nil {
			reg = registry.NewRegistry(docker, registry.RegistryConfig{
				ContainerName: cname,
				HostName:      registryHost,
				HostPort:      fmt.Sprintf("%d", registryPort),
				DataVolume:    dataDir,
			})
		}

		// Set up skopeo for add image functionality
		skopeo := registry.NewSkopeo(registry.SkopeoConfig{
			InsecurePolicy: insecure,
		})

		err = tui.RunWithFullFeatures(client, reg, docker, cname, skopeo, insecure)
		exitOnError(err)
	},
}

func init() {
	tuiCmd.Flags().StringVar(&tuiContainerName, "container", "", "Registry container name")
	rootCmd.AddCommand(tuiCmd)
}
