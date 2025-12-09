package cmd

import (
	"fmt"
	"os"

	"localregistry/internal/registry"

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
	Use:   "repo <repository>",
	Short: "Delete an entire repository",
	Long: `Delete an entire repository and all its tags from the registry.

This command:
1. Deletes all tags/manifests via the registry API
2. Removes the repository directory from the registry storage
3. Optionally runs garbage collection to reclaim disk space

Examples:
  localregistry delete repo myapp
  localregistry delete repo org/myapp
  localregistry delete repo myapp --gc=false`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]

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
			reg := registry.NewRegistry(docker, registry.RegistryConfig{
				ContainerName: containerName,
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
	Use:   "image <repository> [tags...]",
	Short: "Delete specific tags from a repository",
	Long: `Delete specific tags from a repository. If no tags are specified,
all tags in the repository are deleted.

Examples:
  # Delete specific tags
  localregistry delete image myapp v1.0 v1.1

  # Delete all tags from a repository
  localregistry delete image myapp

  # Delete with garbage collection
  localregistry delete image myapp v1.0 --gc`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]
		tagsToDelete := args[1:]

		client, err := newClient()
		exitOnError(err)

		ctx := newContext()

		// If no tags specified, get all tags
		if len(tagsToDelete) == 0 {
			tags, err := client.ListAllTags(ctx, repo)
			exitOnError(err)

			if len(tags) == 0 {
				fmt.Printf("No tags found for repository %s\n", repo)
				return
			}

			tagsToDelete = tags
			fmt.Printf("Deleting all %d tag(s) from %s\n", len(tags), repo)
		}

		// Delete each tag
		deletedCount := 0
		for _, tag := range tagsToDelete {
			fmt.Printf("Deleting %s:%s...\n", repo, tag)
			if err := client.DeleteManifestByTag(ctx, repo, tag); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting %s:%s: %v\n", repo, tag, err)
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

			reg := registry.NewRegistry(docker, registry.RegistryConfig{
				ContainerName: containerName,
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
	Use:   "tag <repository> <tag>",
	Short: "Delete a specific tag",
	Long: `Delete a specific tag from a repository.

This is an alias for 'localregistry manifest delete <repo> <tag>'.

Examples:
  localregistry delete tag myapp v1.0
  localregistry delete tag org/myapp latest`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]
		tag := args[1]

		client, err := newClient()
		exitOnError(err)

		ctx := newContext()

		fmt.Printf("Deleting %s:%s...\n", repo, tag)
		if err := client.DeleteManifestByTag(ctx, repo, tag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Tag %s:%s deleted\n", repo, tag)

		// Run garbage collection if requested
		if deleteRunGC {
			docker, err := registry.NewDockerClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not create Docker client for GC: %v\n", err)
				return
			}

			reg := registry.NewRegistry(docker, registry.RegistryConfig{
				ContainerName: containerName,
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
	deleteCmd.PersistentFlags().StringVar(&containerName, "container", "registry", "Registry container name (for GC)")

	// Add subcommands
	deleteCmd.AddCommand(deleteRepoCmd)
	deleteCmd.AddCommand(deleteImageCmd)
	deleteCmd.AddCommand(deleteTagCmd)

	rootCmd.AddCommand(deleteCmd)
}
