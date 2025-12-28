package cli

import (
	"context"
	"fmt"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	"github.com/ozacod/cpx/internal/pkg/build/cmake"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
	"github.com/spf13/cobra"
)

func ListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project dependencies or targets",
		Long:  "List the dependencies declared in the project or available build targets.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args)
		},
		Args: cobra.NoArgs,
	}

	cmd.Flags().Bool("targets", false, "List build targets instead of dependencies")

	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	projectType := DetectProjectType()
	var builder build.BuildSystem

	switch projectType {
	case ProjectTypeBazel:
		builder = bazel.New()
	case ProjectTypeMeson:
		builder = meson.New()
	case ProjectTypeCMake:
		builder = cmake.New()
	case ProjectTypeVcpkg:
		builder = vcpkg.New()
	default:
		return fmt.Errorf("unknown project type: %s", projectType)
	}

	showTargets, _ := cmd.Flags().GetBool("targets")

	if showTargets {
		targets, err := builder.ListTargets(context.Background())
		if err != nil {
			return fmt.Errorf("failed to list targets: %w", err)
		}

		if len(targets) == 0 {
			fmt.Printf("No targets found for %s.\n", builder.Name())
			return nil
		}

		fmt.Printf("%sTargets (%s):%s\n", colors.Cyan, builder.Name(), colors.Reset)
		for _, t := range targets {
			fmt.Printf("  %s\n", t)
		}
		return nil
	}

	// List dependencies (default)
	deps, err := builder.ListDependencies(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list dependencies: %w", err)
	}

	if len(deps) == 0 {
		fmt.Println("No dependencies found.")
		return nil
	}

	fmt.Printf("%sDependencies (%s):%s\n", colors.Cyan, builder.Name(), colors.Reset)
	for _, dep := range deps {
		version := dep.Version
		if version == "" {
			version = "latest/unknown"
		}
		fmt.Printf("  - %s%s%s @ %s%s%s\n", colors.Green, dep.Name, colors.Reset, colors.Yellow, version, colors.Reset)
	}

	return nil
}
