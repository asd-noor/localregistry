package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
)

var rootCmd = &cobra.Command{
	Use:   "localregistry",
	Short: "Local Docker Registry CLI",
	Long: `A command-line tool for interacting with Docker Registry HTTP API V2.

Provides commands to list repositories, tags, manifests, and blobs,
as well as delete operations for registry management.

Environment variables:
  REGISTRY_URL   Registry URL (default: http://localhost:5000)
  REGISTRY_USER  Username for basic auth
  REGISTRY_PASS  Password for basic auth`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("localregistry %s (%s)\n", Version, Commit)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&registryURL, "url", "u", envOrDefault("REGISTRY_URL", "http://localhost:5000"), "Registry URL")
	rootCmd.PersistentFlags().StringVar(&username, "user", os.Getenv("REGISTRY_USER"), "Username for basic auth")
	rootCmd.PersistentFlags().StringVar(&password, "pass", os.Getenv("REGISTRY_PASS"), "Password for basic auth")
	rootCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "k", false, "Skip TLS certificate verification")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Request timeout")

	rootCmd.AddCommand(versionCmd)
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
