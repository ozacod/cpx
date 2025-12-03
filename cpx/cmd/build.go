package cmd

import (
	"github.com/ozacod/cpx/internal/build"
	"github.com/spf13/cobra"
)

var setupVcpkgEnvFunc func() error

// NewBuildCmd creates the build command
func NewBuildCmd(setupVcpkgEnv func() error) *cobra.Command {
	setupVcpkgEnvFunc = setupVcpkgEnv

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compile the project with CMake",
		Long: `Compile the project with CMake. Supports optimization levels (-O0/1/2/3/s/fast) and clean builds.

Examples:
  cpx build              # Debug build
  cpx build --release    # Release build with O2
  cpx build -O3          # Maximum optimization
  cpx build -j 8         # Use 8 parallel jobs
  cpx build --clean      # Clean rebuild
  cpx build --watch      # Watch for changes and rebuild`,
		RunE: runBuild,
	}

	cmd.Flags().BoolP("release", "r", false, "Build in release mode (O2)")
	cmd.Flags().Bool("debug", false, "Build in debug mode (O0, default)")
	cmd.Flags().IntP("jobs", "j", 0, "Number of parallel jobs (0 = auto)")
	cmd.Flags().String("target", "", "Specific target to build")
	cmd.Flags().BoolP("clean", "c", false, "Clean build directory before building")
	cmd.Flags().StringP("opt", "O", "", "Optimization level: 0, 1, 2, 3, s, fast")
	cmd.Flags().BoolP("watch", "w", false, "Watch for file changes and rebuild automatically")

	return cmd
}

func runBuild(cmd *cobra.Command, args []string) error {
	release, _ := cmd.Flags().GetBool("release")
	jobs, _ := cmd.Flags().GetInt("jobs")
	target, _ := cmd.Flags().GetString("target")
	clean, _ := cmd.Flags().GetBool("clean")
	optLevel, _ := cmd.Flags().GetString("opt")
	watch, _ := cmd.Flags().GetBool("watch")

	if watch {
		return build.WatchAndBuild(release, jobs, target, optLevel, setupVcpkgEnvFunc)
	}

	return build.BuildProject(release, jobs, target, clean, optLevel, setupVcpkgEnvFunc)
}

// Build is kept for backward compatibility (if needed)
func Build(args []string, setupVcpkgEnv func() error) {
	// This function is deprecated - use NewBuildCmd instead
	// Kept for compatibility during migration
}
