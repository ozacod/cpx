package cli

import (
	"fmt"
	"os"

	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/ozacod/cpx/internal/pkg/vcpkg"
	"github.com/spf13/cobra"
)

// TestCmd creates the test command
func TestCmd(client *vcpkg.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Build and run tests",
		Long:  "Build the project tests and run them. Detects vcpkg/CMake or Bazel projects automatically.",
		Example: `  cpx test                 # Build + run all tests
  cpx test --verbose       # Show verbose output
  cpx test --filter MySuite.*`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTest(cmd, args, client)
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show verbose test output")
	cmd.Flags().String("filter", "", "Filter tests by name (ctest regex or bazel target)")
	cmd.Flags().String("toolchain", "", "Toolchain to run tests in (from cpx-ci.yaml)")

	return cmd
}

func runTest(cmd *cobra.Command, args []string, client *vcpkg.Client) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	filter, _ := cmd.Flags().GetString("filter")
	toolchain, _ := cmd.Flags().GetString("toolchain")

	// If toolchain is specified, run tests in Docker via CI
	if toolchain != "" {
		// Note: Filter is currently not passed to CI tests
		if filter != "" {
			fmt.Printf("%sWarning: --filter is currently ignored when running with --toolchain%s\n", Yellow, Reset)
		}
		return runToolchainBuild(toolchain, false, false, true, false)
	}

	// Detect project type
	projectType := DetectProjectType()

	switch projectType {
	case ProjectTypeBazel:
		return runBazelTest(verbose, filter)
	case ProjectTypeMeson:
		return runMesonTest(verbose, filter)
	default:
		// CMake/vcpkg
		return build.RunTests(verbose, filter, client)
	}
}

func runBazelTest(verbose bool, filter string) error {
	fmt.Printf("%sRunning Bazel tests...%s\n", Cyan, Reset)

	bazelArgs := []string{"test"}

	// Add filter if provided (bazel target pattern)
	if filter != "" {
		bazelArgs = append(bazelArgs, filter)
	} else {
		bazelArgs = append(bazelArgs, "//...")
	}

	// Add verbose flag
	if verbose {
		bazelArgs = append(bazelArgs, "--test_output=all")
	} else {
		bazelArgs = append(bazelArgs, "--test_output=errors")
		// Use hidden symlinks (.bazel-bin, .bazel-out, etc.)
		bazelArgs = append(bazelArgs, "--noshow_progress", "--symlink_prefix=.bazel-")
	}

	testCmd := execCommand("bazel", bazelArgs...)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("bazel test failed: %w", err)
	}

	fmt.Printf("%s✓ Tests passed%s\n", Green, Reset)
	return nil
}

func runMesonTest(verbose bool, filter string) error {
	fmt.Printf("%sRunning Meson tests...%s\n", Cyan, Reset)

	// Ensure builddir exists
	if _, err := os.Stat("builddir"); os.IsNotExist(err) {
		// Need to setup first
		if err := runMesonBuild(false, "", false, verbose, "", ""); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
	}

	mesonArgs := []string{"test", "-C", "builddir"}

	// Exclude subproject tests (google-benchmark, gtest, etc.)
	// Only run tests from the main project
	mesonArgs = append(mesonArgs, "--no-suite", "google-benchmark")
	mesonArgs = append(mesonArgs, "--no-suite", "gtest")
	mesonArgs = append(mesonArgs, "--no-suite", "gmock")
	mesonArgs = append(mesonArgs, "--no-suite", "catch2")

	if verbose {
		mesonArgs = append(mesonArgs, "-v")
	} else {
		mesonArgs = append(mesonArgs, "--quiet")
	}

	if filter != "" {
		mesonArgs = append(mesonArgs, filter)
	}

	testCmd := execCommand("meson", mesonArgs...)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("meson test failed: %w", err)
	}

	fmt.Printf("%s✓ Tests passed%s\n", Green, Reset)
	return nil
}
