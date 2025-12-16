package cli

import (
	"fmt"
	"strings"

	"github.com/ozacod/cpx/internal/app/cli/tui"
	"github.com/ozacod/cpx/pkg/config"
	"github.com/spf13/cobra"
)

// AddToolchainCmd creates the add-toolchain command
func AddToolchainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-toolchain",
		Short: "Add a new toolchain to cpx-ci.yaml",
		Long:  "Interactive wizard to add a new build toolchain configuration to cpx-ci.yaml.",
		RunE:  runAddToolchainCmd,
	}

	return cmd
}

// RmToolchainCmd creates the rm-toolchain command
func RmToolchainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm-toolchain [toolchain...]",
		Short: "Remove toolchain(s) from cpx-ci.yaml",
		Long:  "Remove one or more build toolchains from cpx-ci.yaml configuration.",
		RunE:  runRemoveToolchainCmd,
	}

	// Add list subcommand to rm-toolchain
	listRemoveToolchainsCmd := &cobra.Command{
		Use:   "list",
		Short: "List all toolchains in cpx-ci.yaml and select to remove",
		Long:  "List all toolchains defined in cpx-ci.yaml and lets you choose which to remove.",
		RunE:  runListRemoveToolchainsCmd,
	}
	cmd.AddCommand(listRemoveToolchainsCmd)

	return cmd
}

// runAddToolchainCmd adds a build toolchain to cpx-ci.yaml using interactive TUI
func runAddToolchainCmd(_ *cobra.Command, args []string) error {
	// Load existing cpx-ci.yaml or create new one
	ciConfig, err := config.LoadToolchains("cpx-ci.yaml")
	if err != nil {
		// Create new config
		ciConfig = &config.ToolchainConfig{
			Toolchains: []config.Toolchain{},
			Build: config.ToolchainBuild{
				Type:         "Release",
				Optimization: "2",
				Jobs:         0,
			},
			Output: ".bin/ci",
		}
	}

	// Get existing toolchain names as a slice
	var existingToolchainNames []string
	for _, t := range ciConfig.Toolchains {
		existingToolchainNames = append(existingToolchainNames, t.Name)
	}

	// Run interactive TUI (pass existing toolchains for validation)
	toolchainConfig, err := tui.RunAddTargetTUI(existingToolchainNames)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if toolchainConfig == nil {
		// User cancelled
		return nil
	}

	// Convert to CITarget and add
	toolchain := toolchainConfig.ToCITarget()
	ciConfig.Toolchains = append(ciConfig.Toolchains, toolchain)

	// Save cpx-ci.yaml
	if err := config.SaveToolchains(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	fmt.Printf("\n%s+ Added toolchain: %s%s\n", Green, toolchainConfig.Name, Reset)
	fmt.Printf("%sSaved cpx-ci.yaml with %d toolchain(s)%s\n", Green, len(ciConfig.Toolchains), Reset)
	return nil
}

// runRemoveToolchainCmd removes toolchains from cpx-ci.yaml
func runRemoveToolchainCmd(_ *cobra.Command, args []string) error {
	// Load existing cpx-ci.yaml
	ciConfig, err := config.LoadToolchains("cpx-ci.yaml")
	if err != nil {
		return fmt.Errorf("failed to load cpx-ci.yaml: %w\n  No cpx-ci.yaml file found in current directory", err)
	}

	if len(ciConfig.Toolchains) == 0 {
		fmt.Printf("%sNo toolchains in cpx-ci.yaml to remove%s\n", Yellow, Reset)
		return nil
	}

	// If no args, use interactive mode
	if len(args) == 0 {
		// simple interactive mode
		fmt.Printf("%sToolchains in cpx-ci.yaml:%s\n", Cyan, Reset)
		for i, t := range ciConfig.Toolchains {
			fmt.Printf("  %d. %s\n", i+1, t.Name)
		}

		fmt.Printf("\n%sEnter toolchain numbers to remove (comma-separated, or 'all'):%s ", Cyan, Reset)
		var input string
		_, _ = fmt.Scanln(&input)

		var selectedToRemove []string

		if strings.ToLower(strings.TrimSpace(input)) == "all" {
			for _, t := range ciConfig.Toolchains {
				selectedToRemove = append(selectedToRemove, t.Name)
			}
		} else {
			parts := strings.Split(input, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				var idx int
				if _, err := fmt.Sscanf(part, "%d", &idx); err == nil {
					if idx >= 1 && idx <= len(ciConfig.Toolchains) {
						selectedToRemove = append(selectedToRemove, ciConfig.Toolchains[idx-1].Name)
					}
				}
			}
		}

		if len(selectedToRemove) == 0 {
			fmt.Printf("%sNo toolchains selected for removal%s\n", Yellow, Reset)
			return nil
		}

		// Proceed with removal using selectedToRemove
		args = selectedToRemove
	}

	// Build set of toolchains to remove
	toRemove := make(map[string]bool)
	for _, arg := range args {
		toRemove[arg] = true
	}

	// Filter out removed toolchains
	var newTargets []config.Toolchain
	var removed []string
	for _, t := range ciConfig.Toolchains {
		if toRemove[t.Name] {
			removed = append(removed, t.Name)
		} else {
			newTargets = append(newTargets, t)
		}
	}

	if len(removed) == 0 {
		fmt.Printf("%sNo matching toolchains found to remove%s\n\n", Yellow, Reset)
		fmt.Printf("Available toolchains in cpx-ci.yaml:\n")
		for _, t := range ciConfig.Toolchains {
			fmt.Printf("  - %s\n", t.Name)
		}
		return nil
	}

	// Update and save config
	ciConfig.Toolchains = newTargets
	if err := config.SaveToolchains(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	for _, name := range removed {
		fmt.Printf("%s- Removed toolchain: %s%s\n", Red, name, Reset)
	}
	fmt.Printf("\n%sSaved cpx-ci.yaml with %d toolchain(s)%s\n", Green, len(ciConfig.Toolchains), Reset)
	return nil
}

// runListRemoveToolchainsCmd shows all toolchains in cpx-ci.yaml and lets user select to remove
func runListRemoveToolchainsCmd(_ *cobra.Command, _ []string) error {
	// Load existing cpx-ci.yaml
	ciConfig, err := config.LoadToolchains("cpx-ci.yaml")
	if err != nil {
		return fmt.Errorf("failed to load cpx-ci.yaml: %w\n  No cpx-ci.yaml file found in current directory", err)
	}

	if len(ciConfig.Toolchains) == 0 {
		fmt.Printf("%sNo toolchains in cpx-ci.yaml to remove%s\n", Yellow, Reset)
		return nil
	}

	// Build toolchains list for TUI
	var items []tui.ToolchainItem
	for _, t := range ciConfig.Toolchains {
		items = append(items, tui.ToolchainItem{
			Name:     t.Name,
			Platform: describePlatform(t.Name),
		})
	}

	// Run interactive TUI
	selectedToRemove, err := tui.RunToolchainSelection(items, nil, "Select Toolchains to Remove")
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if len(selectedToRemove) == 0 {
		fmt.Printf("%sNo toolchains selected for removal%s\n", Yellow, Reset)
		return nil
	}

	// Remove selected toolchains
	toRemove := make(map[string]bool)
	for _, name := range selectedToRemove {
		toRemove[name] = true
	}

	var newTargets []config.Toolchain
	for _, t := range ciConfig.Toolchains {
		if !toRemove[t.Name] {
			newTargets = append(newTargets, t)
		}
	}

	ciConfig.Toolchains = newTargets
	if err := config.SaveToolchains(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	for name := range toRemove {
		fmt.Printf("%s- Removed toolchain: %s%s\n", Red, name, Reset)
	}
	fmt.Printf("\n%sSaved cpx-ci.yaml with %d toolchain(s)%s\n", Green, len(ciConfig.Toolchains), Reset)
	return nil
}

// describePlatform returns a human-readable platform description
func describePlatform(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return ""
	}
	os := parts[0]
	arch := parts[1]

	osNames := map[string]string{
		"linux": "Linux",
	}
	archNames := map[string]string{
		"amd64": "x86_64",
		"arm64": "ARM64",
	}

	osName := osNames[os]
	if osName == "" {
		osName = os
	}
	archName := archNames[arch]
	if archName == "" {
		archName = arch
	}

	return osName + " " + archName
}
