package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	"github.com/ozacod/cpx/internal/pkg/build/cmake"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
	"github.com/spf13/cobra"
)

func RemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove",
		Short:   "Remove a dependency",
		Long:    "Remove a dependency from your project.",
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(cmd, args)
		},
		Args: cobra.MinimumNArgs(1),
	}

	return cmd
}

func runRemove(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("argument required (pkg1 pkg2 ...)")
	}

	projectType := DetectProjectType()

	// Get the appropriate builder for the project type
	var builder build.BuildSystem

	switch projectType {
	case ProjectTypeVcpkg:
		builder = vcpkg.New()
	case ProjectTypeBazel:
		builder = bazel.New()
	case ProjectTypeMeson:
		builder = meson.New()
	case ProjectTypeCMake:
		builder = cmake.New()
	default:
		return fmt.Errorf("unsupported project type")
	}

	// Remove each dependency
	for _, pkgName := range args {
		if strings.HasPrefix(pkgName, "-") {
			continue
		}
		if err := builder.RemoveDependency(context.Background(), pkgName); err != nil {
			fmt.Printf("%s✗ Failed to remove %s: %v%s\n", colors.Red, pkgName, err, colors.Reset)
			continue
		}
	}

	if projectType == ProjectTypeVcpkg {
		fmt.Printf("Run 'cpx install' or 'cpx build' to update installed packages.\n")
	}

	return nil
}
