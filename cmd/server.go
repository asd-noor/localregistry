package cmd

import (
	"fmt"
	"os"

	"local-registry/internal/registry"

	"github.com/spf13/cobra"
)

var (
	containerName string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the local registry server container",
	Long:  "Commands for managing the local Docker registry container lifecycle.",
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the registry container",
	Long:  "Start the local Docker registry container. Creates it if it doesn't exist.",
	Run: func(cmd *cobra.Command, args []string) {
		docker, err := registry.NewDockerClient()
		exitOnError(err)

		// Use config value if flag wasn't explicitly set
		cname := containerName
		if !cmd.Flags().Changed("container") && cfg != nil {
			cname = cfg.Registry.ContainerName
		}

		// Get data directory from config
		dataDir := ""
		if cfg != nil {
			dataDir = cfg.Registry.DataDir
		}

		reg := registry.NewRegistry(docker, registry.RegistryConfig{
			ContainerName: cname,
			HostPort:      fmt.Sprintf("%d", registryPort),
			HostName:      registryHost,
			DataVolume:    dataDir,
		})

		ctx := newContext()
		if err := reg.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Registry started at %s\n", reg.Address())
	},
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the registry container",
	Long:  "Stop the running local Docker registry container.",
	Run: func(cmd *cobra.Command, args []string) {
		docker, err := registry.NewDockerClient()
		exitOnError(err)

		// Use config value if flag wasn't explicitly set
		cname := containerName
		if !cmd.Flags().Changed("container") && cfg != nil {
			cname = cfg.Registry.ContainerName
		}

		reg := registry.NewRegistry(docker, registry.RegistryConfig{
			ContainerName: cname,
		})

		ctx := newContext()
		if err := reg.Stop(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Registry stopped")
	},
}

var serverRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the registry container",
	Long:  "Stop and start the local Docker registry container.",
	Run: func(cmd *cobra.Command, args []string) {
		docker, err := registry.NewDockerClient()
		exitOnError(err)

		// Use config value if flag wasn't explicitly set
		cname := containerName
		if !cmd.Flags().Changed("container") && cfg != nil {
			cname = cfg.Registry.ContainerName
		}

		reg := registry.NewRegistry(docker, registry.RegistryConfig{
			ContainerName: cname,
		})

		ctx := newContext()
		if err := reg.Restart(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Registry restarted")
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show registry container status",
	Long:  "Display the current status of the local Docker registry container.",
	Run: func(cmd *cobra.Command, args []string) {
		docker, err := registry.NewDockerClient()
		exitOnError(err)

		// Use config value if flag wasn't explicitly set
		cname := containerName
		if !cmd.Flags().Changed("container") && cfg != nil {
			cname = cfg.Registry.ContainerName
		}

		reg := registry.NewRegistry(docker, registry.RegistryConfig{
			ContainerName: cname,
		})

		ctx := newContext()
		status, err := reg.Status(ctx)
		exitOnError(err)

		fmt.Printf("Status: %s\n", status)
	},
}

var serverInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show detailed registry container information",
	Long:  "Display detailed information about the local Docker registry container.",
	Run: func(cmd *cobra.Command, args []string) {
		docker, err := registry.NewDockerClient()
		exitOnError(err)

		// Use config value if flag wasn't explicitly set
		cname := containerName
		if !cmd.Flags().Changed("container") && cfg != nil {
			cname = cfg.Registry.ContainerName
		}

		reg := registry.NewRegistry(docker, registry.RegistryConfig{
			ContainerName: cname,
		})

		ctx := newContext()
		info, err := reg.Info(ctx)
		exitOnError(err)

		fmt.Printf("Container ID:   %s\n", info.ContainerID)
		fmt.Printf("Container Name: %s\n", info.ContainerName)
		fmt.Printf("Image:          %s\n", info.Image)
		fmt.Printf("Status:         %s\n", info.Status)
		fmt.Printf("Running:        %v\n", info.Running)
		fmt.Printf("Started At:     %s\n", info.StartedAt)
		fmt.Printf("Address:        %s\n", info.Address)
		if len(info.Ports) > 0 {
			fmt.Println("Ports:")
			for _, p := range info.Ports {
				fmt.Printf("  - %s\n", p)
			}
		}
		if len(info.Mounts) > 0 {
			fmt.Println("Mounts:")
			for _, m := range info.Mounts {
				fmt.Printf("  - %s\n", m)
			}
		}
	},
}

var (
	gcDeleteUntagged bool
)

var serverGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Run garbage collection",
	Long: `Run garbage collection on the registry to reclaim disk space.

This executes the registry's built-in garbage collector to remove
unreferenced blobs and layers. Use --delete-untagged to also remove
manifests that are not currently tagged.

Example:
  local-registry server gc
  local-registry server gc --delete-untagged`,
	Run: func(cmd *cobra.Command, args []string) {
		docker, err := registry.NewDockerClient()
		exitOnError(err)

		// Use config value if flag wasn't explicitly set
		cname := containerName
		if !cmd.Flags().Changed("container") && cfg != nil {
			cname = cfg.Registry.ContainerName
		}

		reg := registry.NewRegistry(docker, registry.RegistryConfig{
			ContainerName: cname,
		})

		ctx := newContext()
		fmt.Println("Running garbage collection...")

		result, err := reg.GarbageCollect(ctx, gcDeleteUntagged)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			if result != nil && result.Stderr != "" {
				fmt.Fprintf(os.Stderr, "Stderr: %s\n", result.Stderr)
			}
			os.Exit(1)
		}

		if result.Output != "" {
			fmt.Print(result.Output)
		}
		fmt.Println("Garbage collection completed successfully")
	},
}

func init() {
	// Add persistent flag for container name to server command
	// Empty default - falls back to config value (cfg.Registry.ContainerName)
	serverCmd.PersistentFlags().StringVar(&containerName, "container", "", "Registry container name")

	// Add gc-specific flags
	serverGCCmd.Flags().BoolVar(&gcDeleteUntagged, "delete-untagged", true, "Delete untagged manifests")

	// Add subcommands
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverRestartCmd)
	serverCmd.AddCommand(serverStatusCmd)
	serverCmd.AddCommand(serverInfoCmd)
	serverCmd.AddCommand(serverGCCmd)

	rootCmd.AddCommand(serverCmd)
}
