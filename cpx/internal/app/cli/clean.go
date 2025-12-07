package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// CleanCmd creates the clean command
func CleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove build artifacts",
		Long:  "Remove build artifacts. Use --all to also remove generated files.",
		RunE:  runClean,
	}

	cmd.Flags().Bool("all", false, "Also remove generated files")

	return cmd
}

func runClean(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")

	buildDir := "build"
	if _, err := os.Stat(buildDir); err == nil {
		fmt.Printf("%s Cleaning build directory...%s\n", Cyan, Reset)
		if err := os.RemoveAll(buildDir); err != nil {
			return fmt.Errorf("failed to remove build directory: %w", err)
		}
		fmt.Printf("%s Cleaned build directory%s\n", Green, Reset)
	}

	if all {
		dirsToRemove := []string{"out", "build-*"}
		for _, pattern := range dirsToRemove {
			if pattern == "build-*" {
				// Remove all build-* directories
				entries, err := os.ReadDir(".")
				if err == nil {
					for _, entry := range entries {
						if entry.IsDir() {
							matched, _ := filepath.Match("build-*", entry.Name())
							if matched {
								fmt.Printf("%s Removing %s...%s\n", Cyan, entry.Name(), Reset)
								os.RemoveAll(entry.Name())
							}
						}
					}
				}
			} else {
				if _, err := os.Stat(pattern); err == nil {
					fmt.Printf("%s Removing %s...%s\n", Cyan, pattern, Reset)
					os.RemoveAll(pattern)
				}
			}
		}
		fmt.Printf("%s Cleaned all generated files%s\n", Green, Reset)
	}

	return nil
}
