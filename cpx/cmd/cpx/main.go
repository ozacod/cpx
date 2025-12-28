package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/ozacod/cpx/internal/app/cli"
	"github.com/ozacod/cpx/internal/app/cli/root"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
)

func main() {
	rootCmd := root.GetRootCmd()

	rootCmd.AddCommand(cli.BuildCmd())
	rootCmd.AddCommand(cli.RunCmd())
	rootCmd.AddCommand(cli.TestCmd())
	rootCmd.AddCommand(cli.BenchCmd())
	rootCmd.AddCommand(cli.CleanCmd())
	rootCmd.AddCommand(cli.NewCmd())
	rootCmd.AddCommand(cli.AddCmd())
	rootCmd.AddCommand(cli.RemoveCmd())
	rootCmd.AddCommand(cli.ListCmd())
	rootCmd.AddCommand(cli.SearchCmd())
	rootCmd.AddCommand(cli.InfoCmd())
	rootCmd.AddCommand(cli.FmtCmd())
	rootCmd.AddCommand(cli.LintCmd())
	rootCmd.AddCommand(cli.FlawfinderCmd())
	rootCmd.AddCommand(cli.CppcheckCmd())
	rootCmd.AddCommand(cli.AnalyzeCmd())

	rootCmd.AddCommand(cli.DocCmd())
	rootCmd.AddCommand(cli.ReleaseCmd())
	rootCmd.AddCommand(cli.UpgradeCmd())
	rootCmd.AddCommand(cli.ConfigCmd())
	rootCmd.AddCommand(cli.WorkflowCmd())
	rootCmd.AddCommand(cli.HooksCmd())
	rootCmd.AddCommand(cli.UpdateCmd())

	// Toolchain, Runner management (simplified design)
	rootCmd.AddCommand(cli.AddToolchainCmd())
	rootCmd.AddCommand(cli.AddRunnerCmd())
	rootCmd.AddCommand(cli.RmToolchainCmd())
	rootCmd.AddCommand(cli.RmRunnerCmd())

	// Handle vcpkg passthrough for specific commands only,
	// Only forward: install, remove, add-port
	if len(os.Args) > 1 {
		command := os.Args[1]
		// Skip version/help flags - cobra handles these
		if command != "-v" && command != "--version" && command != "version" &&
			command != "-h" && command != "--help" && command != "help" {
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
