package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ozacod/cpx/internal/build/bazel"
	"github.com/ozacod/cpx/internal/build/cmake"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/build/meson"
	"github.com/ozacod/cpx/internal/build/vcpkg"
	"github.com/ozacod/cpx/internal/utils"
	"github.com/ozacod/cpx/internal/utils/colors"
	"github.com/spf13/cobra"
)

func InfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <package>",
		Short: "Show detailed library information",
		Long:  "Show detailed library information for a vcpkg package.",
		RunE:  runInfo,
		Args:  cobra.MinimumNArgs(1),
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

func runInfo(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	packageName := args[0]

	projectType := utils.DetectProjectType()

	var builder build.BuildSystem

	switch projectType {
	case utils.ProjectTypeBazel:
		builder = bazel.New()
	case utils.ProjectTypeMeson:
		builder = meson.New()
	case utils.ProjectTypeCMake:
		builder = cmake.New()
	case utils.ProjectTypeVcpkg:
		builder = vcpkg.New()
	default:
		return fmt.Errorf("unsupported project type %q (supported: vcpkg, bazel, meson, cmake)\n  hint: run 'cpx new' to create a project", projectType)
	}

	info, err := builder.DependencyInfo(packageName)
	if err != nil {
		return err
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	// Print formatted output
	fmt.Printf("%s📦 %s%s %s%s%s\n", colors.Bold, colors.Cyan, info.Name, colors.Yellow, info.Version, colors.Reset)

	if info.Description != "" {
		// Handle multi-line description
		lines := strings.Split(info.Description, "\n")
		for _, line := range lines {
			fmt.Printf("   %s\n", line)
		}
	}

	if info.Homepage != "" {
		fmt.Printf("\n%s🔗 Homepage:%s %s\n", colors.Bold, colors.Reset, info.Homepage)
	}

	if info.License != "" {
		fmt.Printf("%s📄 License:%s  %s\n", colors.Bold, colors.Reset, info.License)
	}

	// Dependencies
	if len(info.Dependencies) > 0 {
		fmt.Printf("\n%s📚 Dependencies:%s\n", colors.Bold, colors.Reset)
		for _, dep := range info.Dependencies {
			fmt.Printf("   • %s\n", dep)
		}
	}

	return nil
}
