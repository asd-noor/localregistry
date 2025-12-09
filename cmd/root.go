package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"localregistry/config"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
)

// cfg holds the loaded configuration
var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "localregistry",
	Short: "Local Docker Registry CLI",
	Long: `A command-line tool for interacting with Docker Registry HTTP API V2.

Provides commands to list repositories, tags, manifests, and blobs,
as well as delete operations for registry management.

Configuration is loaded from (in order of precedence):
  1. Command-line flags
  2. Environment variables (LR_REGISTRY_HOST, LR_REGISTRY_PORT, etc.)
  3. Config file (~/.config/localregistry/config.yaml)
  4. Embedded defaults`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration before any command runs
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		slog.Debug("configuration loaded",
			"host", cfg.Registry.Host,
			"port", cfg.Registry.Port,
			"insecure", cfg.Registry.Insecure,
			"timeout", cfg.Registry.Timeout,
		)

		// Apply config values as defaults for flags not explicitly set
		if !cmd.Flags().Changed("host") {
			registryHost = cfg.Registry.Host
		}
		if !cmd.Flags().Changed("port") {
			registryPort = cfg.Registry.Port
		}
		if !cmd.Flags().Changed("user") {
			username = cfg.Registry.Username
		}
		if !cmd.Flags().Changed("pass") {
			password = cfg.Registry.Password
		}
		if !cmd.Flags().Changed("insecure") {
			insecure = cfg.Registry.Insecure
		}
		if !cmd.Flags().Changed("timeout") {
			timeout = cfg.Registry.Timeout
		}

		slog.Debug("effective registry settings",
			"host", registryHost,
			"port", registryPort,
			"has_credentials", username != "",
			"insecure", insecure,
			"timeout", timeout,
		)

		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("localregistry %s (%s)\n", Version, Commit)
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show configuration information",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, err := config.ConfigFilePath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
		fmt.Printf("Config file: %s\n", configPath)
		fmt.Printf("\nCurrent settings:\n")
		fmt.Printf("  Registry Host: %s\n", registryHost)
		fmt.Printf("  Registry Port: %d\n", registryPort)
		fmt.Printf("  Username:      %s\n", username)
		fmt.Printf("  Insecure:      %v\n", insecure)
		fmt.Printf("  Timeout:       %s\n", timeout)
		if cfg != nil {
			fmt.Printf("  Log Level:     %s\n", cfg.LogLevel)
		}
	},
}

func init() {
	// Define flags with placeholder defaults; actual defaults come from config in PersistentPreRunE
	rootCmd.PersistentFlags().StringVarP(&registryHost, "host", "H", "localhost", "Registry host")
	rootCmd.PersistentFlags().IntVarP(&registryPort, "port", "p", 5000, "Registry port")
	rootCmd.PersistentFlags().StringVar(&username, "user", "", "Username for basic auth")
	rootCmd.PersistentFlags().StringVar(&password, "pass", "", "Password for basic auth")
	rootCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "k", false, "Skip TLS certificate verification")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
}

// GetConfig returns the loaded configuration. May be nil if called before Execute().
func GetConfig() *config.Config {
	return cfg
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
