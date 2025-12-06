package cli

import (
	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/spf13/cobra"
)

var runSetupVcpkgEnvFunc func() error

// NewRunCmd creates the run command
func NewRunCmd(setupVcpkgEnv func() error) *cobra.Command {
	runSetupVcpkgEnvFunc = setupVcpkgEnv

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Build and run the project",
		Long:  "Build and run the project. Additional arguments after -- are passed to the executable.",
		Example: `  cpx run
  cpx run --release
  cpx run --target app -- --flag value`,
		RunE: runRun,
	}

	cmd.Flags().Bool("release", false, "Build in release mode (default is debug)")
	cmd.Flags().String("target", "", "Specific target to run (if multiple executables exist)")

	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	release, _ := cmd.Flags().GetBool("release")
	target, _ := cmd.Flags().GetString("target")

	return build.RunProject(release, target, args, runSetupVcpkgEnvFunc)
}

// Run is kept for backward compatibility (if needed)
func Run(args []string, setupVcpkgEnv func() error) {
	// This function is deprecated - use NewRunCmd instead
	// Kept for compatibility during migration
}
