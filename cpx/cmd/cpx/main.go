package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/ozacod/cpx/internal/app/cli"
	"github.com/ozacod/cpx/internal/app/cli/root"
	"github.com/ozacod/cpx/internal/pkg/vcpkg"
	"github.com/ozacod/cpx/pkg/config"
)

func getBcrPath() string {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return ""
	}
	return cfg.BcrRoot
}

func main() {
	rootCmd := root.GetRootCmd()

	// Initialize vcpkg client
	// We ignore specific initialization errors here because some commands
	// might not need the client (e.g., config, help, version).
	// Commands that strictly need the client should handle checking if it's nil
	// or properly report errors when they try to use it.
	client, _ := vcpkg.NewClient()

	// Register all commands
	rootCmd.AddCommand(cli.BuildCmd(client))
	rootCmd.AddCommand(cli.RunCmd(client))
	rootCmd.AddCommand(cli.TestCmd(client))
	rootCmd.AddCommand(cli.BenchCmd(client))
	rootCmd.AddCommand(cli.CleanCmd())
	rootCmd.AddCommand(cli.NewCmd(client))
	rootCmd.AddCommand(cli.AddCmd(client, getBcrPath))
	rootCmd.AddCommand(cli.RemoveCmd(client))
	rootCmd.AddCommand(cli.ListCmd(client))
	rootCmd.AddCommand(cli.SearchCmd(client))
	rootCmd.AddCommand(cli.InfoCmd(client))
	rootCmd.AddCommand(cli.FmtCmd())
	rootCmd.AddCommand(cli.LintCmd(client))
	rootCmd.AddCommand(cli.FlawfinderCmd())
	rootCmd.AddCommand(cli.CppcheckCmd())
	rootCmd.AddCommand(cli.AnalyzeCmd(client))
	rootCmd.AddCommand(cli.CheckCmd(client))
	rootCmd.AddCommand(cli.DocCmd())
	rootCmd.AddCommand(cli.ReleaseCmd())
	rootCmd.AddCommand(cli.UpgradeCmd())
	rootCmd.AddCommand(cli.ConfigCmd())
	rootCmd.AddCommand(cli.CICmd())
	rootCmd.AddCommand(cli.WorkflowCmd())
	rootCmd.AddCommand(cli.HooksCmd())
	rootCmd.AddCommand(cli.UpdateCmd())

	// Handle vcpkg passthrough for unknown commands
	// Check if command exists before executing
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
			// If not found, try vcpkg passthrough
			if !found {
				if client != nil {
					if err := client.RunCommand(os.Args[1:]); err != nil {
						fmt.Fprintf(os.Stderr, "%sError:%s Failed to run vcpkg command: %v\n", cli.Red, cli.Reset, err)
						fmt.Fprintf(os.Stderr, "Make sure vcpkg is installed and configured: cpx config set-vcpkg-root <path>\n")
						os.Exit(1)
					}
					return
				}
				// If client is nil, we can't run vcpkg command
				fmt.Fprintf(os.Stderr, "%sError:%s Unknown command '%s' and vcpkg client not initialized\n", cli.Red, cli.Reset, command)
				os.Exit(1)
			}
		}
	}

	// Execute root command
	root.Execute()
}
