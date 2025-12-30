package commands

import (
	"fmt"

	"github.com/ozacod/cpx/internal/build/bazel"
	"github.com/ozacod/cpx/internal/build/cmake"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/build/meson"
	"github.com/ozacod/cpx/internal/build/vcpkg"
	"github.com/ozacod/cpx/internal/utils"
	"github.com/spf13/cobra"
)

func CleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clean",
		Aliases: []string{"cl", "cls"},
		Short:   "Remove build artifacts",
		Long: `Remove build artifacts. Automatically detects project type:
  - Bazel: runs 'bazel clean' and removes symlinks (.bin, .out, bazel-*)
  - Meson: removes builddir/
  - CMake/vcpkg: removes build/

Use --all to also remove additional generated files.`,
		Example: `  cpx clean         # Clean build artifacts
  cpx clean --all   # Also remove all generated files`,
		RunE: runClean,
	}

	cmd.Flags().Bool("all", false, "Also remove generated files")

	return cmd
}

func runClean(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")

	projectType := utils.DetectProjectType()
	opts := build.CleanOptions{
		All: all,
	}

	switch projectType {
	case utils.ProjectTypeBazel:
		builder := bazel.New()
		return builder.Clean(opts)
	case utils.ProjectTypeMeson:
		builder := meson.New()
		return builder.Clean(opts)
	case utils.ProjectTypeCMake:
		builder := cmake.New()
		return builder.Clean(opts)
	case utils.ProjectTypeVcpkg:
		builder := vcpkg.New()
		return builder.Clean(opts)
	default:
		return fmt.Errorf("unsupported project type")
	}
}
