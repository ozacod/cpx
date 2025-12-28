package cli

import (
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/internal/pkg/quality"
	"github.com/spf13/cobra"
)

func LintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Run clang-tidy static analysis",
		Long:  "Run clang-tidy static analysis. Use --fix to automatically fix issues.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLint(cmd, args)
		},
	}

	cmd.Flags().BoolP("fix", "f", false, "Automatically fix issues")

	return cmd
}

func runLint(cmd *cobra.Command, args []string) error {
	fix, _ := cmd.Flags().GetBool("fix")
	return quality.LintCode(fix, vcpkg.New())
}
