package commands

import (
	"fmt"

	"github.com/ozacod/cpx/internal/build/bazel"
	"github.com/ozacod/cpx/internal/build/cmake"
	build "github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/build/meson"
	"github.com/ozacod/cpx/internal/build/vcpkg"
	"github.com/ozacod/cpx/internal/utils"
	"github.com/spf13/cobra"
)

func UpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade dependencies to newer versions",
		Long:  "Upgrade dependencies to newer versions. Use 'cpx update' first to see what will be upgraded.",
		RunE:  runUpgrade,
	}

	return cmd
}

func runUpgrade(_ *cobra.Command, _ []string) error {
	projectType, err := utils.RequireProject("upgrade")
	if err != nil {
		return err
	}

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

	return builder.Upgrade()
}
