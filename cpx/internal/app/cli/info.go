package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/vcpkg"
	"github.com/spf13/cobra"
)

// InfoCmd creates the info command
func InfoCmd(client *vcpkg.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <package>",
		Short: "Show detailed library information",
		Long:  "Show detailed library information for a vcpkg package.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo(cmd, args, client)
		},
		Args: cobra.MinimumNArgs(1),
	}

	cmd.Flags().Bool("json", false, "Output in JSON format")

	return cmd
}

// PackageInfo represents the structure of vcpkg x-package-info output
type PackageInfo struct {
	Results map[string]struct {
		Name         string `json:"name"`
		Version      string `json:"version-semver"`
		VersionDate  string `json:"version-date"`
		VersionStr   string `json:"version-string"`
		Description  any    `json:"description"`
		Homepage     string `json:"homepage"`
		License      string `json:"license"`
		Dependencies []any  `json:"dependencies"`
		Features     map[string]struct {
			Description string `json:"description"`
		} `json:"features"`
	} `json:"results"`
}

func runInfo(cmd *cobra.Command, args []string, client *vcpkg.Client) error {
	if client == nil {
		return fmt.Errorf("vcpkg client not initialized")
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	packageName := args[0]

	// Build vcpkg command
	vcpkgArgs := []string{"x-package-info", packageName, "--x-json"}

	// Get vcpkg path
	vcpkgPath, err := client.GetPath()
	if err != nil {
		return fmt.Errorf("failed to get vcpkg path: %w", err)
	}

	// Run vcpkg and capture output
	vcpkgCmd := exec.Command(vcpkgPath, vcpkgArgs...)
	output, err := vcpkgCmd.Output()

	// vcpkg x-package-info returns exit code 1 even on success, so check if we got valid JSON
	if len(output) == 0 && err != nil {
		return fmt.Errorf("failed to get package info: %w", err)
	}

	// Find the JSON part (skip any leading messages like "Fetching registry...")
	jsonStart := strings.Index(string(output), "{")
	if jsonStart == -1 {
		return fmt.Errorf("no package info found for '%s'", packageName)
	}
	jsonData := output[jsonStart:]

	if jsonOutput {
		// Raw JSON output
		fmt.Println(string(jsonData))
		return nil
	}

	// Parse and display nicely
	var info PackageInfo
	if err := json.Unmarshal(jsonData, &info); err != nil {
		return fmt.Errorf("failed to parse package info: %w", err)
	}

	pkg, ok := info.Results[packageName]
	if !ok {
		return fmt.Errorf("package '%s' not found", packageName)
	}

	// Determine version (vcpkg uses different version fields)
	version := pkg.Version
	if version == "" {
		version = pkg.VersionDate
	}
	if version == "" {
		version = pkg.VersionStr
	}

	// Print formatted output
	fmt.Printf("%s📦 %s%s %s%s%s\n", Bold, Cyan, pkg.Name, Yellow, version, Reset)

	// Description can be string or array of strings
	switch desc := pkg.Description.(type) {
	case string:
		fmt.Printf("   %s\n", desc)
	case []interface{}:
		for _, d := range desc {
			if s, ok := d.(string); ok {
				fmt.Printf("   %s\n", s)
			}
		}
	}

	if pkg.Homepage != "" {
		fmt.Printf("\n%s🔗 Homepage:%s %s\n", Bold, Reset, pkg.Homepage)
	}

	if pkg.License != "" {
		fmt.Printf("%s📄 License:%s  %s\n", Bold, Reset, pkg.License)
	}

	// Dependencies
	if len(pkg.Dependencies) > 0 {
		fmt.Printf("\n%s📚 Dependencies:%s\n", Bold, Reset)
		for _, dep := range pkg.Dependencies {
			switch d := dep.(type) {
			case string:
				fmt.Printf("   • %s\n", d)
			case map[string]interface{}:
				if name, ok := d["name"].(string); ok {
					if host, ok := d["host"].(bool); ok && host {
						fmt.Printf("   • %s %s(host)%s\n", name, Dim, Reset)
					} else {
						fmt.Printf("   • %s\n", name)
					}
				}
			}
		}
	}

	// Features
	if len(pkg.Features) > 0 {
		fmt.Printf("\n%s✨ Features:%s\n", Bold, Reset)
		for name, feat := range pkg.Features {
			fmt.Printf("   • %s%s%s: %s\n", Green, name, Reset, feat.Description)
		}
	}

	return nil
}
