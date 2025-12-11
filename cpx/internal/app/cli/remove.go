package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ozacod/cpx/internal/pkg/vcpkg"
	"github.com/spf13/cobra"
)

// RemoveCmd creates the remove command
func RemoveCmd(client *vcpkg.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove",
		Short:   "Remove a dependency",
		Long:    "Remove a dependency. Passes through to vcpkg remove command.",
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(cmd, args, client)
		},
		Args: cobra.MinimumNArgs(1),
	}

	return cmd
}

func runRemove(_ *cobra.Command, args []string, client *vcpkg.Client) error {
	if len(args) == 0 {
		return fmt.Errorf("argument required (pkg1 pkg2 ...)")
	}

	// Check for vcpkg.json (Manifest mode)
	if _, err := os.Stat("vcpkg.json"); err == nil {
		fmt.Printf("%sDetecting manifest mode (vcpkg.json)...%s\n", Green, Reset)

		// Read manifest
		data, err := os.ReadFile("vcpkg.json")
		if err != nil {
			return fmt.Errorf("failed to read vcpkg.json: %w", err)
		}

		// Define manifest structure dynamically
		var manifest map[string]interface{}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("failed to parse vcpkg.json: %w", err)
		}

		// Get dependencies
		deps, ok := manifest["dependencies"]
		if !ok {
			fmt.Printf("%sNo dependencies found in vcpkg.json%s\n", Yellow, Reset)
			return nil
		}

		// Convert dependencies to slice of interface{} to handle mix of strings and objects
		depList, ok := deps.([]interface{})
		if !ok {
			return fmt.Errorf("invalid dependencies format in vcpkg.json")
		}

		// Process removals
		newDeps := make([]interface{}, 0, len(depList))
		removedCount := 0

		for _, dep := range depList {
			depName := ""

			// Handle string dependency: "fmt"
			if str, ok := dep.(string); ok {
				depName = str
			} else if obj, ok := dep.(map[string]interface{}); ok {
				// Handle object dependency: { "name": "fmt", ... }
				if name, ok := obj["name"].(string); ok {
					depName = name
				}
			}

			// Check if this dependency should be removed
			shouldRemove := false
			for _, arg := range args {
				if depName == arg {
					shouldRemove = true
					fmt.Printf("%sRemoving %s from vcpkg.json...%s\n", Green, depName, Reset)
					removedCount++
					break
				}
			}

			if !shouldRemove {
				newDeps = append(newDeps, dep)
			}
		}

		if removedCount == 0 {
			fmt.Printf("%sNo matching dependencies found to remove.%s\n", Yellow, Reset)
			return nil
		}

		// Update manifest
		manifest["dependencies"] = newDeps

		// Write back
		newData, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to encode vcpkg.json: %w", err)
		}

		if err := os.WriteFile("vcpkg.json", newData, 0644); err != nil {
			return fmt.Errorf("failed to write vcpkg.json: %w", err)
		}

		fmt.Printf("%sSuccessfully removed %d dependency(ies)%s\n", Green, removedCount, Reset)
		fmt.Printf("Run 'cpx install' or 'cpx build' to update installed packages.\n")
		return nil
	}

	// Legacy mode (Classic mode)
	// Directly pass all arguments to vcpkg remove command
	// cpx remove <args> -> vcpkg remove <args>
	vcpkgArgs := []string{"remove"}
	vcpkgArgs = append(vcpkgArgs, args...)

	if client == nil {
		return fmt.Errorf("vcpkg client not initialized")
	}
	return client.RunCommand(vcpkgArgs)
}
