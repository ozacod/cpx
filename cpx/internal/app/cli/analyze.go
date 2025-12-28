package cli

import (
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/internal/pkg/quality"
	"github.com/spf13/cobra"
)

func AnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Run comprehensive code analysis and generate HTML report",
		Long:  "Run comprehensive code analysis using cppcheck, clang-tidy, and flawfinder. Generates a combined HTML report (analyze.html).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(cmd, args)
		},
		Args: cobra.ArbitraryArgs,
	}

	cmd.Flags().StringP("output", "o", "analyze.html", "Output HTML file path")
	cmd.Flags().Bool("skip-cppcheck", false, "Skip Cppcheck analysis")
	cmd.Flags().Bool("skip-lint", false, "Skip clang-tidy analysis")
	cmd.Flags().Bool("skip-flawfinder", false, "Skip Flawfinder analysis")

	return cmd
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")
	skipCppcheck, _ := cmd.Flags().GetBool("skip-cppcheck")
	skipLint, _ := cmd.Flags().GetBool("skip-lint")
	skipFlawfinder, _ := cmd.Flags().GetBool("skip-flawfinder")

	// Get remaining args as target directories (default to current directory)
	targets := args
	if len(targets) == 0 {
		targets = []string{"."}
	}

	// quality package needs update too, but for now passing builder logic inside quality
	return quality.RunComprehensiveAnalysis(output, skipCppcheck, skipLint, skipFlawfinder, targets, vcpkg.New())
}
