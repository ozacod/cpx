package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/spf13/cobra"
)

var runSetupVcpkgEnvFunc func() error

// RunCmd creates the run command
func RunCmd(setupVcpkgEnv func() error) *cobra.Command {
	runSetupVcpkgEnvFunc = setupVcpkgEnv

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Build and run the project",
		Long: `Build the project and run the executable. Automatically detects project type:
  - vcpkg/CMake projects: Builds with CMake and runs the binary
  - Bazel projects: Uses bazel run

Arguments after -- are passed to the binary.`,
		Example: `  cpx run                 # Debug build by default
  cpx run --release        # Release build, then run
  cpx run --target app -- --flag value`,
		RunE: runRun,
	}

	cmd.Flags().Bool("release", false, "Build in release mode (-O2). Default is debug")
	cmd.Flags().String("target", "", "Executable target to run (useful if multiple)")
	cmd.Flags().Bool("verbose", false, "Show full build output")

	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	release, _ := cmd.Flags().GetBool("release")
	target, _ := cmd.Flags().GetString("target")
	verbose, _ := cmd.Flags().GetBool("verbose")

	projectType := DetectProjectType()

	switch projectType {
	case ProjectTypeBazel:
		return runBazelRun(release, target, args, verbose)
	case ProjectTypeVcpkg:
		return build.RunProject(release, target, args, verbose, runSetupVcpkgEnvFunc)
	default:
		// Fall back to CMake run even without vcpkg.json
		return build.RunProject(release, target, args, verbose, runSetupVcpkgEnvFunc)
	}
}

func runBazelRun(release bool, target string, args []string, verbose bool) error {
	// Build bazel run args
	bazelArgs := []string{"run"}

	// Add config for release/debug
	if release {
		bazelArgs = append(bazelArgs, "--config=release")
	} else {
		bazelArgs = append(bazelArgs, "--config=debug")
	}

	// Add target or try to find one
	if target != "" {
		if !strings.HasPrefix(target, "//") && !strings.HasPrefix(target, ":") {
			target = "//:" + target
		}
		bazelArgs = append(bazelArgs, target)
	} else {
		// Try to find the main target from BUILD.bazel
		mainTarget, err := findBazelMainTarget()
		if err != nil {
			return fmt.Errorf("no target specified and could not find main target: %w\n  hint: use --target to specify the target", err)
		}
		bazelArgs = append(bazelArgs, mainTarget)
	}

	// Add -- and user args if present
	if len(args) > 0 {
		bazelArgs = append(bazelArgs, "--")
		bazelArgs = append(bazelArgs, args...)
	}

	fmt.Printf("%sRunning with Bazel...%s\n", Cyan, Reset)
	if verbose {
		fmt.Printf("  Running: bazel %v\n", bazelArgs)
	}

	runCmd := exec.Command("bazel", bazelArgs...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	return runCmd.Run()
}

// findBazelMainTarget tries to find a cc_binary target in BUILD.bazel
func findBazelMainTarget() (string, error) {
	// Read BUILD.bazel
	content, err := os.ReadFile("BUILD.bazel")
	if err != nil {
		return "", fmt.Errorf("could not read BUILD.bazel: %w", err)
	}

	// Look for cc_binary declarations
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name = \"") {
			// Extract target name
			name := strings.TrimPrefix(line, "name = \"")
			name = strings.TrimSuffix(name, "\",")
			name = strings.TrimSuffix(name, "\"")
			// Skip library targets (usually end with _lib)
			if !strings.HasSuffix(name, "_lib") && !strings.HasSuffix(name, "_test") {
				return "//:" + name, nil
			}
		}
	}

	// Fallback: use project directory name
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	projectName := filepath.Base(cwd)
	return "//:" + projectName, nil
}
