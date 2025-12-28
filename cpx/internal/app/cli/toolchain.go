package cli

import (
	"fmt"
	"strings"

	"github.com/ozacod/cpx/internal/app/cli/tui"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
	"github.com/ozacod/cpx/pkg/config"
	"github.com/spf13/cobra"
)

func AddToolchainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-toolchain",
		Short: "Add a build configuration (toolchain) to cpx-ci.yaml",
		RunE:  runAddToolchainCmd,
	}
	return cmd
}

func AddRunnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-runner",
		Short: "Add a runner (execution environment) to cpx-ci.yaml",
		RunE:  runAddRunnerCmd,
	}
	return cmd
}

func RmToolchainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm-toolchain [name...]",
		Short: "Remove toolchain(s) from cpx-ci.yaml",
		RunE:  runRemoveToolchainCmd,
	}
	return cmd
}

func RmRunnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm-runner [name...]",
		Short: "Remove runner(s) from cpx-ci.yaml",
		RunE:  runRemoveRunnerCmd,
	}
	return cmd
}

func runAddToolchainCmd(_ *cobra.Command, _ []string) error {
	ciConfig, err := loadOrCreateConfig()
	if err != nil {
		return err
	}

	var existingNames []string
	for _, t := range ciConfig.Toolchains {
		existingNames = append(existingNames, t.Name)
	}
	var runnerNames []string
	for _, r := range ciConfig.Runners {
		runnerNames = append(runnerNames, r.Name)
	}

	// Run TUI (now adds build configuration)
	result, err := tui.RunAddToolchainTUI(existingNames, runnerNames)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	if result == nil {
		return nil // Cancelled
	}

	runnerName := result.Runner

	toolchain := config.Toolchain{
		Name:      result.Name,
		Runner:    runnerName,
		BuildType: result.BuildType,
	}

	ciConfig.Toolchains = append(ciConfig.Toolchains, toolchain)

	if err := config.SaveToolchains(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	fmt.Printf("\n%s✓ Added toolchain: %s%s\n", colors.Green, result.Name, colors.Reset)
	return nil
}

func runAddRunnerCmd(_ *cobra.Command, _ []string) error {
	ciConfig, err := loadOrCreateConfig()
	if err != nil {
		return err
	}

	var existingNames []string
	for _, r := range ciConfig.Runners {
		existingNames = append(existingNames, r.Name)
	}

	result, err := tui.RunAddRunnerTUI(existingNames)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	if result == nil {
		return nil
	}

	runner := config.Runner{
		Name:               result.Name,
		Type:               result.Type,
		Image:              result.Image,
		Host:               result.Host,
		User:               result.User,
		CC:                 result.CC,
		CXX:                result.CXX,
		CMakeToolchainFile: result.CMakeToolchain,
	}

	ciConfig.Runners = append(ciConfig.Runners, runner)

	if err := config.SaveToolchains(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	fmt.Printf("\n%s✓ Added runner: %s (%s)%s\n", colors.Green, result.Name, result.Type, colors.Reset)
	return nil
}

func runRemoveToolchainCmd(_ *cobra.Command, args []string) error {
	ciConfig, err := config.LoadToolchains("cpx-ci.yaml")
	if err != nil {
		return fmt.Errorf("failed to load cpx-ci.yaml: %w", err)
	}

	if len(ciConfig.Toolchains) == 0 {
		fmt.Printf("%sNo toolchains in cpx-ci.yaml%s\n", colors.Yellow, colors.Reset)
		return nil
	}

	if len(args) == 0 {
		fmt.Printf("%sUsage: cpx rm-toolchain <name...>%s\n", colors.Yellow, colors.Reset)
		fmt.Printf("\nAvailable toolchains:\n")
		for _, t := range ciConfig.Toolchains {
			fmt.Printf("  - %s\n", t.Name)
		}
		return nil
	}

	toRemove := make(map[string]bool)
	for _, arg := range args {
		toRemove[arg] = true
	}

	var newItems []config.Toolchain
	var removed []string
	for _, t := range ciConfig.Toolchains {
		if toRemove[t.Name] {
			removed = append(removed, t.Name)
		} else {
			newItems = append(newItems, t)
		}
	}

	if len(removed) == 0 {
		fmt.Printf("%sNo matching toolchains found%s\n", colors.Yellow, colors.Reset)
		return nil
	}

	ciConfig.Toolchains = newItems
	if err := config.SaveToolchains(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	for _, name := range removed {
		fmt.Printf("%s✗ Removed toolchain: %s%s\n", colors.Red, name, colors.Reset)
	}
	return nil
}

func runRemoveRunnerCmd(_ *cobra.Command, args []string) error {
	ciConfig, err := config.LoadToolchains("cpx-ci.yaml")
	if err != nil {
		return fmt.Errorf("failed to load cpx-ci.yaml: %w", err)
	}

	if len(ciConfig.Runners) == 0 {
		fmt.Printf("%sNo runners in cpx-ci.yaml%s\n", colors.Yellow, colors.Reset)
		return nil
	}

	if len(args) == 0 {
		fmt.Printf("%sUsage: cpx rm-runner <name...>%s\n", colors.Yellow, colors.Reset)
		fmt.Printf("\nAvailable runners:\n")
		for _, r := range ciConfig.Runners {
			fmt.Printf("  - %s (%s)\n", r.Name, r.Type)
		}
		return nil
	}

	toRemove := make(map[string]bool)
	for _, arg := range args {
		toRemove[arg] = true
	}

	var newItems []config.Runner
	var removed []string
	for _, r := range ciConfig.Runners {
		if toRemove[r.Name] {
			removed = append(removed, r.Name)
		} else {
			newItems = append(newItems, r)
		}
	}

	if len(removed) == 0 {
		fmt.Printf("%sNo matching runners found%s\n", colors.Yellow, colors.Reset)
		return nil
	}

	ciConfig.Runners = newItems
	if err := config.SaveToolchains(ciConfig, "cpx-ci.yaml"); err != nil {
		return err
	}

	for _, name := range removed {
		fmt.Printf("%s✗ Removed runner: %s%s\n", colors.Red, name, colors.Reset)
	}
	return nil
}

func loadOrCreateConfig() (*config.ToolchainConfig, error) {
	ciConfig, err := config.LoadToolchains("cpx-ci.yaml")
	if err != nil {
		// Create empty config - no defaults
		ciConfig = &config.ToolchainConfig{}
	}
	return ciConfig, nil
}

func utilsSanitizeRunnerName(name string) string {
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ToLower(name)
	return name
}
