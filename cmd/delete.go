package cmd

import (
	"fmt"
	"os"
	"strings"

	"local-registry/internal/registry"

	"github.com/spf13/cobra"
)

var (
	deleteRunGC bool
	deleteForce bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete images, tags, or repositories",
	Long:  "Commands for deleting images, tags, or entire repositories from the registry.",
}

var deleteRepoCmd = &cobra.Command{
	Use:   "repo <image>",
	Short: "Delete an entire repository",
	Long: `Delete an entire repository and all its tags from the registry.

The image reference should be the repository name (tags are ignored):
  - name (e.g., alpine)
  - repository/name (e.g., library/alpine)

This command:
1. Deletes all tags/manifests via the registry API
2. Removes the repository directory from the registry storage
3. Optionally runs garbage collection to reclaim disk space

Examples:
  local-registry delete repo myapp
  local-registry delete repo org/myapp`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Strip any tag - we're deleting the whole repo
		repo := stripTag(args[0])

		client, err := newClient()
		exitOnError(err)

		ctx := newContext()

		// First, get all tags for this repository
		tags, err := client.ListAllTags(ctx, repo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not list tags for %s: %v\n", repo, err)
			tags = []string{}
		}

		// Delete each tag's manifest
		deletedCount := 0
		for _, tag := range tags {
			fmt.Printf("Deleting %s:%s...\n", repo, tag)
			if err := client.DeleteManifestByTag(ctx, repo, tag); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete %s:%s: %v\n", repo, tag, err)
			} else {
				deletedCount++
			}
		}

		if deletedCount > 0 {
			fmt.Printf("Deleted %d tag(s) from %s\n", deletedCount, repo)
		}

		// Remove repository directory via Docker exec
		docker, err := registry.NewDockerClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not create Docker client: %v\n", err)
		} else {
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
				DataVolume:    dataDir,
			})

			fmt.Printf("Removing repository directory: %s\n", repo)
			if err := reg.RemoveRepository(ctx, repo); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not remove repository directory: %v\n", err)
			}

			// Run garbage collection if requested
			if deleteRunGC {
				fmt.Println("Running garbage collection...")
				if _, err := reg.GarbageCollect(ctx, true); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: garbage collection failed: %v\n", err)
				} else {
					fmt.Println("Garbage collection completed")
				}
			}
		}

		fmt.Printf("Repository %s deleted\n", repo)
	},
}

var deleteImageCmd = &cobra.Command{
	Use:   "image <image> [tags...]",
	Short: "Delete specific tags from a repository",
	Long: `Delete specific tags from a repository. If no tags are specified,
all tags in the repository are deleted.

The image reference can include a tag, or tags can be provided as additional arguments:
  - name (deletes all tags)
  - name:tag (deletes specific tag)
  - name tag1 tag2 (deletes multiple tags)

Examples:
  # Delete specific tag
  local-registry delete image myapp:v1.0

  # Delete multiple tags
  local-registry delete image myapp v1.0 v1.1

  # Delete all tags from a repository
  local-registry delete image myapp

  # Delete with garbage collection
  local-registry delete image myapp:v1.0 --gc`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, tag := parseImageRef(args[0])
		var tagsToDelete []string

		// Check if tag was specified in the image reference
		hasInlineTag := strings.Contains(args[0], ":") || strings.Contains(args[0], "@")

		if len(args) > 1 {
			// Additional tags provided as arguments
			tagsToDelete = args[1:]
			if hasInlineTag {
				// Also include the inline tag
				tagsToDelete = append([]string{tag}, tagsToDelete...)
			}
		} else if hasInlineTag {
			// Only inline tag specified
			tagsToDelete = []string{tag}
		}
		// If tagsToDelete is empty, we'll fetch all tags below

		client, err := newClient()
		exitOnError(err)

		ctx := newContext()

		// If no tags specified, get all tags
		if len(tagsToDelete) == 0 {
			tags, err := client.ListAllTags(ctx, name)
			exitOnError(err)

			if len(tags) == 0 {
				fmt.Printf("No tags found for repository %s\n", name)
				return
			}

			tagsToDelete = tags
			fmt.Printf("Deleting all %d tag(s) from %s\n", len(tags), name)
		}

		// Delete each tag
		deletedCount := 0
		for _, t := range tagsToDelete {
			fmt.Printf("Deleting %s:%s...\n", name, t)
			if err := client.DeleteManifestByTag(ctx, name, t); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting %s:%s: %v\n", name, t, err)
				if !deleteForce {
					os.Exit(1)
				}
			} else {
				deletedCount++
			}
		}

		fmt.Printf("Deleted %d tag(s)\n", deletedCount)

		// Run garbage collection if requested
		if deleteRunGC {
			docker, err := registry.NewDockerClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not create Docker client for GC: %v\n", err)
				return
			}

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
				DataVolume:    dataDir,
			})

			fmt.Println("Running garbage collection...")
			if _, err := reg.GarbageCollect(ctx, true); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: garbage collection failed: %v\n", err)
			} else {
				fmt.Println("Garbage collection completed")
			}
		}
	},
}

var deleteTagCmd = &cobra.Command{
	Use:   "tag <image>",
	Short: "Delete a specific tag",
	Long: `Delete a specific tag from a repository.

The image reference must include a tag:
  - name:tag (e.g., alpine:latest)
  - repository/name:tag (e.g., library/alpine:latest)

Examples:
  local-registry delete tag myapp:v1.0
  local-registry delete tag org/myapp:latest`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, tag := parseImageRef(args[0])

		if tag == "latest" && !strings.Contains(args[0], ":") && !strings.Contains(args[0], "@") {
			fmt.Fprintln(os.Stderr, "Error: tag is required. Use format: <image>:<tag>")
			os.Exit(1)
		}

		client, err := newClient()
		exitOnError(err)

		ctx := newContext()

		fmt.Printf("Deleting %s:%s...\n", name, tag)
		if err := client.DeleteManifestByTag(ctx, name, tag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Tag %s:%s deleted\n", name, tag)

		// Run garbage collection if requested
		if deleteRunGC {
			docker, err := registry.NewDockerClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not create Docker client for GC: %v\n", err)
				return
			}

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
				DataVolume:    dataDir,
			})

			fmt.Println("Running garbage collection...")
			if _, err := reg.GarbageCollect(ctx, true); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: garbage collection failed: %v\n", err)
			} else {
				fmt.Println("Garbage collection completed")
			}
		}
	},
}

func init() {
	// Shared flags
	deleteCmd.PersistentFlags().BoolVar(&deleteRunGC, "gc", false, "Run garbage collection after deletion")
	deleteCmd.PersistentFlags().BoolVarP(&deleteForce, "force", "f", false, "Continue on errors")
	deleteCmd.PersistentFlags().StringVar(&containerName, "container", "", "Registry container name (for GC)")

	// Add subcommands
	deleteCmd.AddCommand(deleteRepoCmd)
	deleteCmd.AddCommand(deleteImageCmd)
	deleteCmd.AddCommand(deleteTagCmd)

	rootCmd.AddCommand(deleteCmd)
}
