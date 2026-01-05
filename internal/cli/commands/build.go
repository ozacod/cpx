package commands

import (
	"fmt"

	"github.com/ozacod/cpx/internal/build/bazel"
	"github.com/ozacod/cpx/internal/build/cmake"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/build/meson"
	"github.com/ozacod/cpx/internal/build/vcpkg"
	"github.com/ozacod/cpx/internal/utils"
	"github.com/ozacod/cpx/internal/utils/colors"
	"github.com/spf13/cobra"
)

func BuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "build",
		Aliases: []string{"b"},
		Short:   "Compile the project",
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
		RunE: runBuild,
	}

	cmd.Flags().BoolP("release", "r", false, "Release build (-O2). Default is debug")
	cmd.Flags().Bool("debug", false, "Debug build (-O0). Default; kept for compatibility")
	cmd.Flags().IntP("jobs", "j", 0, "Parallel jobs for build (0 = auto)")
	cmd.Flags().StringP("toolchain", "t", "", "Toolchain to build (from cpx-ci.yaml)")
	cmd.Flags().BoolP("clean", "c", false, "Clean build directory before building")
	cmd.Flags().StringP("opt", "O", "", "Override optimization level: 0,1,2,3,s,fast")
	cmd.Flags().BoolP("verbose", "v", false, "Show full build output")
	cmd.Flags().BoolP("quiet", "q", false, "Quiet mode (only exit code/minimal status)")
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
			toolchainName, _ := cmd.Flags().GetString("toolchain")
			return runToolchainBuild(ToolchainBuildOptions{
				ToolchainName:     toolchainName,
				ExecuteAfterBuild: false,
				RunTests:          false,
				RunBenchmarks:     false,
				Verbose:           true, // Build all is often verbose, or we can get it from flag
			})
		},
	}
	allCmd.Flags().StringP("toolchain", "t", "", "Build only specific toolchain (default: all)")
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

	if toolchain != "" {
		return runToolchainBuild(ToolchainBuildOptions{
			ToolchainName:     toolchain,
			ExecuteAfterBuild: false,
			RunTests:          false,
			RunBenchmarks:     false,
			Verbose:           verbose,
		})
	}

	asan, _ := cmd.Flags().GetBool("asan")
	tsan, _ := cmd.Flags().GetBool("tsan")
	msan, _ := cmd.Flags().GetBool("msan")
	ubsan, _ := cmd.Flags().GetBool("ubsan")

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

	projectType := utils.DetectProjectType()

	utils.WarnMissingBuildTools(projectType)

	list, _ := cmd.Flags().GetBool("list")

	handleList := func(b build.BuildSystem) error {
		targets, err := b.ListTargets()
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

	quiet, _ := cmd.Flags().GetBool("quiet")
	if verbose && quiet {
		return fmt.Errorf("cannot use both --verbose and --quiet")
	}

	outputMode := build.OutputModeUI
	if verbose {
		outputMode = build.OutputModeVerbose
	} else if quiet {
		outputMode = build.OutputModeQuiet
	}

	buildOpts := build.BuildOptions{
		Release:    release,
		OptLevel:   optLevel,
		Sanitizer:  sanitizer,
		Target:     "",
		Jobs:       jobs,
		Clean:      clean,
		OutputMode: outputMode,
	}

	var builder build.BuildSystem
	switch projectType {
	case utils.ProjectTypeBazel:
		builder = bazel.New()
	case utils.ProjectTypeMeson:
		builder = meson.New()
	case utils.ProjectTypeCMake:
		builder = cmake.New()
	case utils.ProjectTypeVcpkg:
		builder = vcpkg.New()
	default:
		return fmt.Errorf("unsupported project type %q (supported: vcpkg, bazel, meson, cmake)\n  hint: run 'cpx new' to create a project", projectType)
	}

	if list {
		return handleList(builder)
	}

	return builder.Build(buildOpts)
}
