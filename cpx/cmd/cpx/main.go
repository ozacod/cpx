package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ozacod/cpx/internal/app/cli"
	"github.com/ozacod/cpx/internal/app/cli/root"
	"github.com/ozacod/cpx/internal/pkg/vcpkg"
)

var vcpkgClient *vcpkg.Client

func getVcpkgClient() (*vcpkg.Client, error) {
	if vcpkgClient == nil {
		var err error
		vcpkgClient, err = vcpkg.NewClient()
		if err != nil {
			return nil, err
		}
	}
	return vcpkgClient, nil
}

func setupVcpkgEnv() error {
	client, err := getVcpkgClient()
	if err != nil {
		return err
	}

	err = client.SetupEnv()
	if err != nil {
		return err
	}

	if os.Getenv("CPX_DEBUG") != "" {
		fmt.Printf("%s[DEBUG] VCPKG Environment:%s\n", cli.Cyan, cli.Reset)
		fmt.Printf("  VCPKG_ROOT=%s\n", os.Getenv("VCPKG_ROOT"))
		fmt.Printf("  VCPKG_FEATURE_FLAGS=%s\n", os.Getenv("VCPKG_FEATURE_FLAGS"))
		fmt.Printf("  VCPKG_DISABLE_REGISTRY_UPDATE=%s\n", os.Getenv("VCPKG_DISABLE_REGISTRY_UPDATE"))
	}

	return nil
}

const (
	Reset  = cli.Reset
	Red    = cli.Red
	Yellow = cli.Yellow
	Cyan   = cli.Cyan
)

func getVcpkgPath() (string, error) {
	client, err := getVcpkgClient()
	if err != nil {
		return "", err
	}
	return client.GetPath()
}

func runVcpkgCommand(args []string) error {
	client, err := getVcpkgClient()
	if err != nil {
		return err
	}
	return client.RunCommand(args)
}

func main() {
	rootCmd := root.GetRootCmd()

	// Register all commands
	rootCmd.AddCommand(cli.BuildCmd(setupVcpkgEnv))
	rootCmd.AddCommand(cli.RunCmd(setupVcpkgEnv))
	rootCmd.AddCommand(cli.TestCmd(setupVcpkgEnv))
	rootCmd.AddCommand(cli.CleanCmd())
	rootCmd.AddCommand(cli.NewCmd(getVcpkgPath, setupVcpkgProject))
	rootCmd.AddCommand(cli.AddCmd(runVcpkgCommand))
	rootCmd.AddCommand(cli.RemoveCmd(runVcpkgCommand))
	rootCmd.AddCommand(cli.ListCmd(runVcpkgCommand))
	rootCmd.AddCommand(cli.SearchCmd(runVcpkgCommand, getVcpkgPath))
	rootCmd.AddCommand(cli.InfoCmd(runVcpkgCommand))
	rootCmd.AddCommand(cli.FmtCmd())
	rootCmd.AddCommand(cli.LintCmd(setupVcpkgEnv, getVcpkgPath))
	rootCmd.AddCommand(cli.FlawfinderCmd())
	rootCmd.AddCommand(cli.CppcheckCmd())
	rootCmd.AddCommand(cli.AnalyzeCmd(setupVcpkgEnv, getVcpkgPath))
	rootCmd.AddCommand(cli.CheckCmd(setupVcpkgEnv))
	rootCmd.AddCommand(cli.DocCmd())
	rootCmd.AddCommand(cli.ReleaseCmd())
	rootCmd.AddCommand(cli.UpgradeCmd())
	rootCmd.AddCommand(cli.ConfigCmd())
	rootCmd.AddCommand(cli.CICmd())
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
				if c.Name() == command || contains(c.Aliases, command) {
					found = true
					break
				}
			}
			// If not found, try vcpkg passthrough
			if !found {
				if err := runVcpkgCommand(os.Args[1:]); err != nil {
					fmt.Fprintf(os.Stderr, "%sError:%s Failed to run vcpkg command: %v\n", Red, Reset, err)
					fmt.Fprintf(os.Stderr, "Make sure vcpkg is installed and configured: cpx config set-vcpkg-root <path>\n")
					os.Exit(1)
				}
				return
			}
		}
	}

	// Execute root command
	root.Execute()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func setupVcpkgProject(targetDir, _ string, _ bool, dependencies []string) error {
	vcpkgPath, err := getVcpkgPath()
	if err != nil {
		return fmt.Errorf("vcpkg not configured: %w\n   Run: cpx config set-vcpkg-root <path>", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(targetDir); err != nil {
		return fmt.Errorf("failed to change to project directory: %w", err)
	}

	fmt.Printf("%s Initializing vcpkg.json...%s\n", Cyan, Reset)

	vcpkgCmd := exec.Command(vcpkgPath, "new", "--application")
	vcpkgCmd.Stdout = os.Stdout
	vcpkgCmd.Stderr = os.Stderr
	vcpkgCmd.Env = os.Environ()
	for i, env := range vcpkgCmd.Env {
		if strings.HasPrefix(env, "VCPKG_ROOT=") {
			vcpkgCmd.Env = append(vcpkgCmd.Env[:i], vcpkgCmd.Env[i+1:]...)
			break
		}
	}
	if err := vcpkgCmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize vcpkg.json: %w", err)
	}

	if len(dependencies) > 0 {
		fmt.Printf("%s Adding dependencies from template...%s\n", Cyan, Reset)
		for _, dep := range dependencies {
			if dep == "" {
				continue
			}
			fmt.Printf("   Adding %s...\n", dep)
			// vcpkg add requires "port" or "artifact" as the second argument
			// We're adding ports (packages), so use "port"
			addCmd := exec.Command(vcpkgPath, "add", "port", dep)
			addCmd.Stdout = os.Stdout
			addCmd.Stderr = os.Stderr
			addCmd.Env = vcpkgCmd.Env // Use same environment
			if err := addCmd.Run(); err != nil {
				fmt.Printf("%s  Warning: Failed to add dependency '%s': %v%s\n", Yellow, dep, err, Reset)
				// Continue with other dependencies even if one fails
			}
		}
	}

	return nil
}
