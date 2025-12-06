package cli

import (
	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/spf13/cobra"
)

var testSetupVcpkgEnvFunc func() error

// NewTestCmd creates the test command
func NewTestCmd(setupVcpkgEnv func() error) *cobra.Command {
	testSetupVcpkgEnvFunc = setupVcpkgEnv

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Build and run tests",
		Long:  "Build and run tests for the project.",
		Example: `  cpx test
  cpx test --verbose
  cpx test --filter MySuite.*`,
		RunE: runTest,
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show verbose output")
	cmd.Flags().String("filter", "", "Filter tests by name (regexp supported by ctest)")

	return cmd
}

func runTest(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	filter, _ := cmd.Flags().GetString("filter")

	return build.RunTests(verbose, filter, testSetupVcpkgEnvFunc)
}

// Test is kept for backward compatibility (if needed)
func Test(args []string, setupVcpkgEnv func() error) {
	// This function is deprecated - use NewTestCmd instead
	// Kept for compatibility during migration
}
