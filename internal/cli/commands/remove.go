package commands

import (
	"fmt"
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

func RemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove [package]",
		Short:   "Remove a dependency",
		Long:    "Remove a dependency from your project.",
		Aliases: []string{"rm"},
		RunE:    runRemove,
		Args:    cobra.MinimumNArgs(1),
	}

	return cmd
}

func runRemove(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("argument required (pkg1 pkg2 ...)")
	}

	projectType := utils.DetectProjectType()

	// Get the appropriate builder for the project type
	var builder build.BuildSystem

	switch projectType {
	case utils.ProjectTypeVcpkg:
		builder = vcpkg.New()
	case utils.ProjectTypeBazel:
		builder = bazel.New()
	case utils.ProjectTypeMeson:
		builder = meson.New()
	case utils.ProjectTypeCMake:
		builder = cmake.New()
	default:
		return fmt.Errorf("unsupported project type")
	}

	// Remove each dependency
	for _, pkgName := range args {
		if strings.HasPrefix(pkgName, "-") {
			continue
		}
		if err := builder.RemoveDependency(pkgName); err != nil {
			fmt.Printf("%s✗ Failed to remove %s: %v%s\n", colors.Red, pkgName, err, colors.Reset)
			continue
		}
	}

	if projectType == utils.ProjectTypeVcpkg {
		fmt.Printf("Run 'cpx install' or 'cpx build' to update installed packages.\n")
	}

	return nil
}
