package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/ozacod/cpx/internal/build/vcpkg"
	"github.com/ozacod/cpx/internal/cli/commands"
	"github.com/ozacod/cpx/internal/cli/root"
	"github.com/ozacod/cpx/internal/utils/colors"
)

func main() {
	rootCmd := root.GetRootCmd()

	rootCmd.AddCommand(commands.BuildCmd())
	rootCmd.AddCommand(commands.RunCmd())
	rootCmd.AddCommand(commands.TestCmd())
	rootCmd.AddCommand(commands.BenchCmd())
	rootCmd.AddCommand(commands.CleanCmd())
	rootCmd.AddCommand(commands.NewCmd())
	rootCmd.AddCommand(commands.AddCmd())
	rootCmd.AddCommand(commands.RemoveCmd())
	rootCmd.AddCommand(commands.ListCmd())
	rootCmd.AddCommand(commands.SearchCmd())
	rootCmd.AddCommand(commands.InfoCmd())
	rootCmd.AddCommand(commands.FmtCmd())
	rootCmd.AddCommand(commands.LintCmd())
	rootCmd.AddCommand(commands.FlawfinderCmd())
	rootCmd.AddCommand(commands.CppcheckCmd())

	rootCmd.AddCommand(commands.DocCmd())
	rootCmd.AddCommand(commands.ReleaseCmd())
	rootCmd.AddCommand(commands.SelfUpgradeCmd())
	rootCmd.AddCommand(commands.ConfigCmd())
	rootCmd.AddCommand(commands.WorkflowCmd())
	rootCmd.AddCommand(commands.HooksCmd())
	rootCmd.AddCommand(commands.UpdateCmd())
	rootCmd.AddCommand(commands.UpgradeCmd())

	// Toolchain, Runner management (simplified design)
	rootCmd.AddCommand(commands.AddToolchainCmd())
	rootCmd.AddCommand(commands.AddRunnerCmd())
	rootCmd.AddCommand(commands.RmToolchainCmd())
	rootCmd.AddCommand(commands.RmRunnerCmd())
	rootCmd.AddCommand(commands.VersionCmd())

	// Handle vcpkg passthrough for specific commands only,
	// Only forward: install, remove, add-port
	if len(os.Args) > 1 {
		command := os.Args[1]
		// Skip help flags - cobra handles these
		if command != "-h" && command != "--help" && command != "help" {
			// Check if it's a known command
			found := false
			for _, c := range rootCmd.Commands() {
				if c.Name() == command || slices.Contains(c.Aliases, command) {
					found = true
					break
				}
			}
			// If not found, check if it's a whitelisted vcpkg command
			if !found {
				// Only allow specific vcpkg commands to be forwarded
				allowedVcpkgCommands := []string{"install", "remove", "add-port"}
				if slices.Contains(allowedVcpkgCommands, command) {
					// Use temporary builder to run vcpkg command
					// Initialize without error check as it might just need PATH
					builder := vcpkg.New()
					if err := builder.RunCommand(os.Args[1:]); err != nil {
						fmt.Fprintf(os.Stderr, "%sError:%s Failed to run vcpkg command: %v\n", colors.Red, colors.Reset, err)
						fmt.Fprintf(os.Stderr, "Make sure vcpkg is installed and configured: cpx config set-vcpkg-root <path>\n")
						os.Exit(1)
					}
					return
				}
				// Unknown command - let cobra handle it (will show help)
			}
		}
	}

	root.Execute()
}
