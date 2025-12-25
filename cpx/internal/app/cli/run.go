package cli

import (
	"context"
	"fmt"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/spf13/cobra"
)

// RunCmd creates the run command
func RunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Build and run the project",
		Long: `Build the project and run the executable. Automatically detects project type:
  - vcpkg/CMake projects: Builds with CMake and runs the binary
  - Bazel projects: Uses bazel run

Arguments after -- are passed to the binary.`,
		Example: `  cpx run                 # Debug build by default
  cpx run --release        # Release build, then run
  cpx run --asan           # Run with AddressSanitizer
  cpx run --target app -- --flag value`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, args)
		},
	}

	cmd.Flags().Bool("release", false, "Build in release mode (-O2). Default is debug")
	cmd.Flags().String("toolchain", "", "Toolchain to run in Docker (from cpx-ci.yaml)")
	cmd.Flags().StringP("opt", "O", "", "Override optimization level: 0,1,2,3,s,fast")
	cmd.Flags().Bool("verbose", false, "Show full build output")
	// Sanitizer flags
	cmd.Flags().Bool("asan", false, "Run with AddressSanitizer")
	cmd.Flags().Bool("tsan", false, "Run with ThreadSanitizer")
	cmd.Flags().Bool("msan", false, "Run with MemorySanitizer")
	cmd.Flags().Bool("ubsan", false, "Run with UndefinedBehaviorSanitizer")

	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	release, _ := cmd.Flags().GetBool("release")
	toolchain, _ := cmd.Flags().GetString("toolchain")
	optLevel, _ := cmd.Flags().GetString("opt")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// --toolchain is for CI builds (Docker)
	if toolchain != "" {
		return runToolchainBuild(ToolchainBuildOptions{
			ToolchainName:     toolchain,
			Rebuild:           false,
			ExecuteAfterBuild: true,
			RunTests:          false,
			RunBenchmarks:     false,
		})
	}

	// Parse sanitizer flags
	asan, _ := cmd.Flags().GetBool("asan")
	tsan, _ := cmd.Flags().GetBool("tsan")
	msan, _ := cmd.Flags().GetBool("msan")
	ubsan, _ := cmd.Flags().GetBool("ubsan")

	// Validate only one sanitizer is used
	sanitizer := ""
	sanitizerCount := 0
	if asan {
		sanitizer = "asan"
		sanitizerCount++
	}
	if tsan {
		sanitizer = "tsan"
		sanitizerCount++
	}
	if msan {
		sanitizer = "msan"
		sanitizerCount++
	}
	if ubsan {
		sanitizer = "ubsan"
		sanitizerCount++
	}
	if sanitizerCount > 1 {
		return fmt.Errorf("only one sanitizer can be used at a time (got %d)", sanitizerCount)
	}

	projectType := DetectProjectType()

	// Check for missing build tools and warn the user
	WarnMissingBuildTools(projectType)

	opts := build.RunOptions{
		Release:   release,
		OptLevel:  optLevel,
		Sanitizer: sanitizer,
		Target:    "",
		Args:      args,
		Verbose:   verbose,
	}

	switch projectType {
	case ProjectTypeBazel:
		builder := bazel.New()
		return builder.Run(context.Background(), opts)
	case ProjectTypeMeson:
		builder := meson.New()
		return builder.Run(context.Background(), opts)
	case ProjectTypeVcpkg:
		builder := vcpkg.New()
		return builder.Run(context.Background(), opts)
	default:
		return fmt.Errorf("unsupported project type")
	}
}
