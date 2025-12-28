package cli

import (
	"context"
	"fmt"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	"github.com/ozacod/cpx/internal/pkg/build/cmake"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/pkg/config"
	"github.com/spf13/cobra"
)

func AddCmd() *cobra.Command {
	// Set the BCR path provider for bazel builder
	bazel.SetBCRPathProvider(func() string {
		cfg, err := config.LoadGlobal()
		if err != nil {
			return ""
		}
		return cfg.BcrRoot
	})

	cmd := &cobra.Command{
		Use:   "add [package]",
		Short: "Add a dependency",
		Long: `Add a dependency to your project.

For vcpkg projects: passes through to 'vcpkg add port' and prints usage info.
For Bazel projects: fetches the latest version from BCR and updates MODULE.bazel.
For Meson projects: uses 'meson wrap install' to add from WrapDB.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, args)
		},
		Args: cobra.MinimumNArgs(1),
	}

	return cmd
}

func runAdd(_ *cobra.Command, args []string) error {
	projectType, err := RequireProject("cpx add")
	if err != nil {
		return err
	}

	name := args[0]
	version := ""
	if len(args) > 1 {
		version = args[1]
	}

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

	return builder.AddDependency(context.Background(), name, version)
}
