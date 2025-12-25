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

// BuildCmd creates the build command
func BuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compile the project",
		Long: `Compile the project. Automatically detects project type:
  - vcpkg/CMake projects: Uses CMake with vcpkg toolchain
  - Bazel projects: Uses bazel build`,
		Example: `  cpx build              # Debug build (default)
  cpx build --release    # Release build (-O2)
  cpx build -O3          # Maximum optimization
  cpx build -j 8         # Use 8 parallel jobs
  cpx build --clean      # Clean rebuild
  cpx build --asan       # Build with AddressSanitizer
  cpx build --tsan       # Build with ThreadSanitizer
  cpx build all          # Build all toolchains (Docker)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd, args)
		},
	}

	cmd.Flags().BoolP("release", "r", false, "Release build (-O2). Default is debug")
	cmd.Flags().Bool("debug", false, "Debug build (-O0). Default; kept for compatibility")
	cmd.Flags().IntP("jobs", "j", 0, "Parallel jobs for build (0 = auto)")
	cmd.Flags().String("toolchain", "", "Toolchain to build (from cpx-ci.yaml)")
	cmd.Flags().BoolP("clean", "c", false, "Clean build directory before building")
	cmd.Flags().StringP("opt", "O", "", "Override optimization level: 0,1,2,3,s,fast")
	cmd.Flags().Bool("verbose", false, "Show full build output")
	// Sanitizer flags
	cmd.Flags().Bool("asan", false, "Build with AddressSanitizer")
	cmd.Flags().Bool("tsan", false, "Build with ThreadSanitizer")
	cmd.Flags().Bool("msan", false, "Build with MemorySanitizer")
	cmd.Flags().Bool("ubsan", false, "Build with UndefinedBehaviorSanitizer")
	cmd.Flags().Bool("list", false, "List available build targets")

	//todo: all should be tested
	allCmd := &cobra.Command{
		Use:   "all",
		Short: "Build all toolchains using Docker",
		Long:  "Build for all toolchains defined in cpx-ci.yaml using Docker containers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			rebuild, _ := cmd.Flags().GetBool("rebuild")
			toolchainName, _ := cmd.Flags().GetString("toolchain")
			return runToolchainBuild(ToolchainBuildOptions{
				ToolchainName:     toolchainName,
				Rebuild:           rebuild,
				ExecuteAfterBuild: false,
				RunTests:          false,
				RunBenchmarks:     false,
			})
		},
	}
	allCmd.Flags().String("toolchain", "", "Build only specific toolchain (default: all)")
	allCmd.Flags().Bool("rebuild", false, "Rebuild Docker images even if they exist")
	cmd.AddCommand(allCmd)

	return cmd
}

func runBuild(cmd *cobra.Command, _ []string) error {
	release, _ := cmd.Flags().GetBool("release")
	jobs, _ := cmd.Flags().GetInt("jobs")
	toolchain, _ := cmd.Flags().GetString("toolchain")
	clean, _ := cmd.Flags().GetBool("clean")
	optLevel, _ := cmd.Flags().GetString("opt")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// --toolchain is for CI builds (Docker)
	// Use `cpx build all --toolchain <name>` for the same behavior
	if toolchain != "" {
		return runToolchainBuild(ToolchainBuildOptions{
			ToolchainName:     toolchain,
			Rebuild:           false,
			ExecuteAfterBuild: false,
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

	// ... sanitizer logic ...

	list, _ := cmd.Flags().GetBool("list")

	// Helper to handle listing targets
	handleList := func(b build.BuildSystem) error {
		targets, err := b.ListTargets(context.Background())
		if err != nil {
			return fmt.Errorf("failed to list targets: %w", err)
		}
		if len(targets) == 0 {
			fmt.Printf("No targets found for %s.\n", b.Name())
			return nil
		}
		fmt.Printf("%sListing %s targets...%s\n", colors.Cyan, b.Name(), colors.Reset)
		for _, t := range targets {
			fmt.Printf("  %s\n", t)
		}
		return nil
	}

	buildOpts := build.BuildOptions{
		Release:   release,
		OptLevel:  optLevel,
		Sanitizer: sanitizer,
		Target:    "",
		Jobs:      jobs,
		Clean:     clean,
		Verbose:   verbose,
	}

	// Helper to create builder based on type
	var builder build.BuildSystem
	switch projectType {
	case ProjectTypeBazel:
		builder = bazel.New()
	case ProjectTypeMeson:
		builder = meson.New()
	default:
		builder = vcpkg.New()
	}

	if list {
		return handleList(builder)
	}

	return builder.Build(context.Background(), buildOpts)
}
