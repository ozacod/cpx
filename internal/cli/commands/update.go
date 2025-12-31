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

func UpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for outdated dependencies",
		Long:  "Check for outdated dependencies. Shows packages that have newer versions available.",
		RunE:  runUpdate,
	}

	return cmd
}

func runUpdate(_ *cobra.Command, _ []string) error {
	projectType, err := utils.RequireProject("update")
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
		return fmt.Errorf("unsupported project type")
	}

	return builder.Update()
}
