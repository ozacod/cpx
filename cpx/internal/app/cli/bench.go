package cli

import (
	"fmt"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	"github.com/ozacod/cpx/internal/pkg/build/cmake"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/spf13/cobra"
)

func BenchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bench",
		Aliases: []string{"be"},
		Short:   "Build and run benchmarks",
		Long:    "Build the project benchmarks and run them. Detects vcpkg/CMake or Bazel projects automatically.",
		Example: `  cpx bench            # Build + run all benchmarks
  cpx bench --verbose  # Show verbose output
  cpx bench --target //bench:myapp_bench  # Run specific benchmark (Bazel)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBenchCmd(cmd)
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show verbose build output")
	cmd.Flags().String("target", "", "Specific benchmark target to run (Bazel projects)")
	cmd.Flags().StringP("toolchain", "t", "", "Toolchain to run benchmarks in (from cpx-ci.yaml)")

	return cmd
}

func runBenchCmd(cmd *cobra.Command) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	target, _ := cmd.Flags().GetString("target")
	toolchain, _ := cmd.Flags().GetString("toolchain")

	if toolchain != "" {
		return runToolchainBuild(ToolchainBuildOptions{
			ToolchainName:     toolchain,
			ExecuteAfterBuild: false,
			RunTests:          false,
			RunBenchmarks:     true,
			Verbose:           verbose,
		})
	}
	projectType := DetectProjectType()

	opts := build.BenchOptions{
		Verbose: verbose,
		Target:  target,
	}

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
		return fmt.Errorf("unsupported project type")
	}
	return builder.Bench(opts)
}
