package cli

import (
	"context"
	"fmt"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
	"github.com/spf13/cobra"
)

// TestCmd creates the test command
func TestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Build and run tests",
		Long:  "Build the project tests and run them. Detects vcpkg/CMake or Bazel projects automatically.",
		Example: `  cpx test                 # Build + run all tests
  cpx test --verbose       # Show verbose output
  cpx test --filter MySuite.*`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTest(cmd, args)
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show verbose test output")
	cmd.Flags().String("filter", "", "Filter tests by name (ctest regex or bazel target)")
	cmd.Flags().String("toolchain", "", "Toolchain to run tests in (from cpx-ci.yaml)")

	return cmd
}

func runTest(cmd *cobra.Command, _ []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	filter, _ := cmd.Flags().GetString("filter")
	toolchain, _ := cmd.Flags().GetString("toolchain")

	// If toolchain is specified, run tests in Docker via CI
	if toolchain != "" {
		if filter != "" {
			fmt.Printf("%sWarning: --filter is currently ignored when running with --toolchain%s\n", colors.Yellow, colors.Reset)
		}
		return runToolchainBuild(ToolchainBuildOptions{
			ToolchainName:     toolchain,
			Rebuild:           false,
			ExecuteAfterBuild: false,
			RunTests:          true,
			RunBenchmarks:     false,
		})
	}

	// Detect project type and get builder
	projectType := DetectProjectType()

	var builder build.BuildSystem

	switch projectType {
	case ProjectTypeBazel:
		builder = bazel.New()
	case ProjectTypeMeson:
		builder = meson.New()
	case ProjectTypeVcpkg:
		builder = vcpkg.New()
	default:
		return fmt.Errorf("could not detect project type (no MODULE.bazel, meson.build, or vcpkg.json found)")
	}

	opts := build.TestOptions{
		Verbose: verbose,
		Filter:  filter,
	}

	return builder.Test(context.Background(), opts)
}
