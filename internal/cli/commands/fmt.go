package commands

import (
	"github.com/ozacod/cpx/internal/quality"
	"github.com/spf13/cobra"
)

func FmtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fmt",
		Aliases: []string{"format"},
		Short:   "Format code with clang-format",
		Long:    "Format code with clang-format. Use --check to verify formatting without modifying files.",
		RunE:    runFmt,
	}

	cmd.Flags().BoolP("check", "c", false, "Check formatting without modifying files")

	return cmd
}

func runFmt(cmd *cobra.Command, _ []string) error {
	check, _ := cmd.Flags().GetBool("check")
	return quality.FormatCode(check)
}
