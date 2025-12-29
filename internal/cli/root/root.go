package root

import (
	"os"

	"github.com/ozacod/cpx/internal/cli/commands"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cpx",
	Short: "Cargo-like DX for modern C++ projects",
	Long: `cpx - Cargo-like DX for modern C++

Generate, build, lint, test, and ship CMake/vcpkg-based C++ projects with sensible defaults and cross-compilation ready Docker targets.`,
	Version: commands.Version,
	// Don't show usage on errors by default
	SilenceUsage:  true,
	SilenceErrors: true, // handle printing ourselves in Execute
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		commands.PrintError("%v", err)
		os.Exit(1)
	}
}

func GetRootCmd() *cobra.Command {
	return rootCmd
}
