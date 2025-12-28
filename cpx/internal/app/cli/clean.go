package cli

import (
	"context"
	"fmt"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	"github.com/ozacod/cpx/internal/pkg/build/cmake"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
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

	projectType := DetectProjectType()
	opts := build.CleanOptions{
		All: all,
	}

	switch projectType {
	case ProjectTypeBazel:
		builder := bazel.New()
		return builder.Clean(context.Background(), opts)
	case ProjectTypeMeson:
		builder := meson.New()
		return builder.Clean(context.Background(), opts)
	case ProjectTypeCMake:
		builder := cmake.New()
		return builder.Clean(context.Background(), opts)
	case ProjectTypeVcpkg:
		builder := vcpkg.New()
		return builder.Clean(context.Background(), opts)
	default:
		return fmt.Errorf("unsupported project type")
	}
}
