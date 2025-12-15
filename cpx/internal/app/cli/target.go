package cli

import (
	"fmt"
	"strings"

	"github.com/ozacod/cpx/internal/app/cli/tui"
	"github.com/ozacod/cpx/pkg/config"
	"github.com/spf13/cobra"
)

// AddTargetCmd creates the add-target command (promoted from ci add-target)
func AddTargetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-target",
		Short: "Add a new CI build target to cpx-ci.yaml",
		Long:  "Interactive wizard to add a new build target configuration to cpx-ci.yaml.",
		RunE:  runAddTargetCmd,
	}

	return cmd
}

// RmTargetCmd creates the rm-target command (promoted from ci rm-target)
func RmTargetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm-target [target...]",
		Short: "Remove build target(s) from cpx-ci.yaml",
		Long:  "Remove one or more build targets from cpx-ci.yaml configuration.",
		RunE:  runRemoveTargetCmd,
	}

	// Add list subcommand to rm-target
	listRemoveTargetsCmd := &cobra.Command{
		Use:   "list",
		Short: "List all targets in cpx-ci.yaml and select to remove",
		Long:  "List all targets defined in cpx-ci.yaml and lets you choose which to remove.",
		RunE:  runListRemoveTargetsCmd,
	}
	cmd.AddCommand(listRemoveTargetsCmd)

	return cmd
}

// runAddTargetCmd adds a build target to cpx-ci.yaml using interactive TUI
func runAddTargetCmd(_ *cobra.Command, args []string) error {
	// Load existing cpx-ci.yaml or create new one
	ciConfig, err := config.LoadCI("cpx-ci.yaml")
	if err != nil {
		// Create new config
		ciConfig = &config.CIConfig{
			Targets: []config.CITarget{},
			Build: config.CIBuild{
				Type:         "Release",
				Optimization: "2",
				Jobs:         0,
			},
			Output: ".bin/ci",
		}
	}

	// Get existing target names as a slice
	var existingTargetNames []string
	for _, t := range ciConfig.Targets {
		existingTargetNames = append(existingTargetNames, t.Name)
	}

	// Run interactive TUI (pass existing targets for validation)
	targetConfig, err := tui.RunAddTargetTUI(existingTargetNames)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if targetConfig == nil {
		// User cancelled
		return nil
	}

	// Convert to CITarget and add
	target := targetConfig.ToCITarget()
	ciConfig.Targets = append(ciConfig.Targets, target)

	// Save cpx-ci.yaml
	if err := config.SaveCI(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	fmt.Printf("\n%s+ Added target: %s%s\n", Green, targetConfig.Name, Reset)
	fmt.Printf("%sSaved cpx-ci.yaml with %d target(s)%s\n", Green, len(ciConfig.Targets), Reset)
	return nil
}

// runRemoveTargetCmd removes targets from cpx-ci.yaml
func runRemoveTargetCmd(_ *cobra.Command, args []string) error {
	// Load existing cpx-ci.yaml
	ciConfig, err := config.LoadCI("cpx-ci.yaml")
	if err != nil {
		return fmt.Errorf("failed to load cpx-ci.yaml: %w\n  No cpx-ci.yaml file found in current directory", err)
	}

	if len(ciConfig.Targets) == 0 {
		fmt.Printf("%sNo targets in cpx-ci.yaml to remove%s\n", Yellow, Reset)
		return nil
	}

	// If no args, use interactive mode
	if len(args) == 0 {
		// simple interactive mode
		fmt.Printf("%sTargets in cpx-ci.yaml:%s\n", Cyan, Reset)
		for i, t := range ciConfig.Targets {
			fmt.Printf("  %d. %s\n", i+1, t.Name)
		}

		fmt.Printf("\n%sEnter target numbers to remove (comma-separated, or 'all'):%s ", Cyan, Reset)
		var input string
		fmt.Scanln(&input)

		var selectedToRemove []string

		if strings.ToLower(strings.TrimSpace(input)) == "all" {
			for _, t := range ciConfig.Targets {
				selectedToRemove = append(selectedToRemove, t.Name)
			}
		} else {
			parts := strings.Split(input, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				var idx int
				if _, err := fmt.Sscanf(part, "%d", &idx); err == nil {
					if idx >= 1 && idx <= len(ciConfig.Targets) {
						selectedToRemove = append(selectedToRemove, ciConfig.Targets[idx-1].Name)
					}
				}
			}
		}

		if len(selectedToRemove) == 0 {
			fmt.Printf("%sNo targets selected for removal%s\n", Yellow, Reset)
			return nil
		}

		// Proceed with removal using selectedToRemove
		args = selectedToRemove
	}

	// Build set of targets to remove
	toRemove := make(map[string]bool)
	for _, arg := range args {
		toRemove[arg] = true
	}

	// Filter out removed targets
	var newTargets []config.CITarget
	var removed []string
	for _, t := range ciConfig.Targets {
		if toRemove[t.Name] {
			removed = append(removed, t.Name)
		} else {
			newTargets = append(newTargets, t)
		}
	}

	if len(removed) == 0 {
		fmt.Printf("%sNo matching targets found to remove%s\n\n", Yellow, Reset)
		fmt.Printf("Available targets in cpx-ci.yaml:\n")
		for _, t := range ciConfig.Targets {
			fmt.Printf("  - %s\n", t.Name)
		}
		return nil
	}

	// Update and save config
	ciConfig.Targets = newTargets
	if err := config.SaveCI(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	for _, name := range removed {
		fmt.Printf("%s- Removed target: %s%s\n", Red, name, Reset)
	}
	fmt.Printf("\n%sSaved cpx-ci.yaml with %d target(s)%s\n", Green, len(ciConfig.Targets), Reset)
	return nil
}

// runListRemoveTargetsCmd shows all targets in cpx-ci.yaml and lets user select to remove
func runListRemoveTargetsCmd(_ *cobra.Command, _ []string) error {
	// Load existing cpx-ci.yaml
	ciConfig, err := config.LoadCI("cpx-ci.yaml")
	if err != nil {
		return fmt.Errorf("failed to load cpx-ci.yaml: %w\n  No cpx-ci.yaml file found in current directory", err)
	}

	if len(ciConfig.Targets) == 0 {
		fmt.Printf("%sNo targets in cpx-ci.yaml to remove%s\n", Yellow, Reset)
		return nil
	}

	// Build targets list for TUI
	var targets []tui.Target
	for _, t := range ciConfig.Targets {
		targets = append(targets, tui.Target{
			Name:     t.Name,
			Platform: describePlatform(t.Name),
		})
	}

	// Run interactive TUI
	selectedToRemove, err := tui.RunTargetSelection(targets, nil, "Select Targets to Remove")
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if len(selectedToRemove) == 0 {
		fmt.Printf("%sNo targets selected for removal%s\n", Yellow, Reset)
		return nil
	}

	// Remove selected targets
	toRemove := make(map[string]bool)
	for _, name := range selectedToRemove {
		toRemove[name] = true
	}

	var newTargets []config.CITarget
	for _, t := range ciConfig.Targets {
		if !toRemove[t.Name] {
			newTargets = append(newTargets, t)
		}
	}

	ciConfig.Targets = newTargets
	if err := config.SaveCI(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	for name := range toRemove {
		fmt.Printf("%s- Removed target: %s%s\n", Red, name, Reset)
	}
	fmt.Printf("\n%sSaved cpx-ci.yaml with %d target(s)%s\n", Green, len(ciConfig.Targets), Reset)
	return nil
}
