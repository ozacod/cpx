package cli

import (
	"fmt"
	"os"

	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/spf13/cobra"
)

var testSetupVcpkgEnvFunc func() error

// TestCmd creates the test command
func TestCmd(setupVcpkgEnv func() error) *cobra.Command {
	testSetupVcpkgEnvFunc = setupVcpkgEnv

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Build and run tests",
		Long:  "Build the project tests and run them. Detects vcpkg/CMake or Bazel projects automatically.",
		Example: `  cpx test                 # Build + run all tests
  cpx test --verbose       # Show verbose output
  cpx test --filter MySuite.*`,
		RunE: runTest,
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show verbose test output")
	cmd.Flags().String("filter", "", "Filter tests by name (ctest regex or bazel target)")

	return cmd
}

func runTest(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	filter, _ := cmd.Flags().GetString("filter")

	// Detect project type
	projectType := DetectProjectType()

	switch projectType {
	case ProjectTypeBazel:
		return runBazelTest(verbose, filter)
	case ProjectTypeMeson:
		return runMesonTest(verbose, filter)
	default:
		// CMake/vcpkg
		return build.RunTests(verbose, filter, testSetupVcpkgEnvFunc)
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
		bazelArgs = append(bazelArgs, "--noshow_progress")
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
		if err := runMesonBuild(false, "", false, verbose); err != nil {
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
