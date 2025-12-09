package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/spf13/cobra"
)

var setupVcpkgEnvFunc func() error

// BuildCmd creates the build command
func BuildCmd(setupVcpkgEnv func() error) *cobra.Command {
	setupVcpkgEnvFunc = setupVcpkgEnv

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
  cpx build --watch      # Watch for changes and rebuild`,
		RunE: runBuild,
	}

	cmd.Flags().BoolP("release", "r", false, "Release build (-O2). Default is debug")
	cmd.Flags().Bool("debug", false, "Debug build (-O0). Default; kept for compatibility")
	cmd.Flags().IntP("jobs", "j", 0, "Parallel jobs for build (0 = auto)")
	cmd.Flags().String("target", "", "Specific target to build")
	cmd.Flags().BoolP("clean", "c", false, "Clean build directory before building")
	cmd.Flags().StringP("opt", "O", "", "Override optimization level: 0,1,2,3,s,fast")
	cmd.Flags().BoolP("watch", "w", false, "Watch for file changes and rebuild automatically")
	cmd.Flags().Bool("verbose", false, "Show full build output")

	return cmd
}

func runBuild(cmd *cobra.Command, args []string) error {
	release, _ := cmd.Flags().GetBool("release")
	jobs, _ := cmd.Flags().GetInt("jobs")
	target, _ := cmd.Flags().GetString("target")
	clean, _ := cmd.Flags().GetBool("clean")
	optLevel, _ := cmd.Flags().GetString("opt")
	watch, _ := cmd.Flags().GetBool("watch")
	verbose, _ := cmd.Flags().GetBool("verbose")

	projectType := DetectProjectType()

	switch projectType {
	case ProjectTypeBazel:
		if watch {
			fmt.Printf("%sWatch mode not yet supported for Bazel projects%s\n", Yellow, Reset)
			return nil
		}
		return runBazelBuild(release, target, clean, verbose)
	case ProjectTypeVcpkg:
		if watch {
			return build.WatchAndBuild(release, jobs, target, optLevel, verbose, setupVcpkgEnvFunc)
		}
		return build.BuildProject(release, jobs, target, clean, optLevel, verbose, setupVcpkgEnvFunc)
	default:
		// Fall back to CMake build even without vcpkg.json
		if watch {
			return build.WatchAndBuild(release, jobs, target, optLevel, verbose, setupVcpkgEnvFunc)
		}
		return build.BuildProject(release, jobs, target, clean, optLevel, verbose, setupVcpkgEnvFunc)
	}
}

func runBazelBuild(release bool, target string, clean bool, verbose bool) error {
	// Clean if requested
	if clean {
		fmt.Printf("%sCleaning Bazel build...%s\n", Cyan, Reset)
		cleanCmd := exec.Command("bazel", "clean")
		cleanCmd.Stdout = os.Stdout
		cleanCmd.Stderr = os.Stderr
		if err := cleanCmd.Run(); err != nil {
			return fmt.Errorf("bazel clean failed: %w", err)
		}
	}

	// Build args
	bazelArgs := []string{"build"}

	// Add config for release/debug
	if release {
		bazelArgs = append(bazelArgs, "--config=release")
	} else {
		bazelArgs = append(bazelArgs, "--config=debug")
	}

	// Add target or default to //...
	if target != "" {
		bazelArgs = append(bazelArgs, target)
	} else {
		bazelArgs = append(bazelArgs, "//...")
	}

	fmt.Printf("%sBuilding with Bazel...%s\n", Cyan, Reset)
	if verbose {
		fmt.Printf("  Running: bazel %v\n", bazelArgs)
	}

	buildCmd := exec.Command("bazel", bazelArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("bazel build failed: %w", err)
	}

	fmt.Printf("%s✓ Build successful%s\n", Green, Reset)
	return nil
}
