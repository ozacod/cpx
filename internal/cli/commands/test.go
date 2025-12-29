package commands

import (
	"fmt"

	"github.com/ozacod/cpx/internal/build/bazel"
	"github.com/ozacod/cpx/internal/build/cmake"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/build/meson"
	"github.com/ozacod/cpx/internal/build/vcpkg"
	"github.com/ozacod/cpx/internal/utils/colors"
	"github.com/spf13/cobra"
)

func TestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "test",
		Aliases: []string{"t"},
		Short:   "Build and run tests",
		Long:    "Build the project tests and run them. Detects vcpkg/CMake or Bazel projects automatically.",
		Example: `  cpx test                 # Build + run all tests
  cpx test --verbose       # Show verbose output
  cpx test --filter MySuite.*`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTest(cmd)
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show verbose test output")
	cmd.Flags().StringP("filter", "f", "", "Filter tests by name (ctest regex or bazel target)")
	cmd.Flags().StringP("toolchain", "t", "", "Toolchain to run tests in (from cpx-ci.yaml)")

	return cmd
}

func runTest(cmd *cobra.Command) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	filter, _ := cmd.Flags().GetString("filter")
	toolchain, _ := cmd.Flags().GetString("toolchain")

	if toolchain != "" {
		if filter != "" {
			fmt.Printf("%sWarning: --filter is currently ignored when running with --toolchain%s\n", colors.Yellow, colors.Reset)
		}
		return runToolchainBuild(ToolchainBuildOptions{
			ToolchainName:     toolchain,
			ExecuteAfterBuild: false,
			RunTests:          true,
			RunBenchmarks:     false,
			Verbose:           verbose,
		})
	}

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
		return fmt.Errorf("could not detect project type (no MODULE.bazel, meson.build, CMakeLists.txt, or vcpkg.json found)")
	}

	opts := build.TestOptions{
		Verbose: verbose,
		Filter:  filter,
	}

	return builder.Test(opts)
}
