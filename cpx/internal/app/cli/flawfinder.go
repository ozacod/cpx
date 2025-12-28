package cli

import (
	"github.com/ozacod/cpx/internal/pkg/quality"
	"github.com/spf13/cobra"
)

func FlawfinderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flawfinder",
		Short: "Run Flawfinder security analysis for C/C++",
		Long:  "Run Flawfinder security analysis for C/C++. Scans C/C++ code for security vulnerabilities.",
		RunE:  runFlawfinder,
		Args:  cobra.ArbitraryArgs,
	}

	cmd.Flags().Int("minlevel", 1, "Minimum risk level to report (0-5, default: 1)")
	cmd.Flags().Bool("csv", false, "Output results in CSV format")
	cmd.Flags().Bool("html", false, "Output results in HTML format")
	cmd.Flags().String("output", "", "Output file path (required for HTML/CSV output)")
	cmd.Flags().Bool("dataflow", false, "Enable dataflow analysis")
	cmd.Flags().Bool("quiet", false, "Quiet mode (minimal output)")
	cmd.Flags().Bool("singleline", false, "Single line output format")
	cmd.Flags().Int("context", 2, "Number of lines of context to show")

	return cmd
}

func runFlawfinder(cmd *cobra.Command, args []string) error {
	minLevel, _ := cmd.Flags().GetInt("minlevel")
	csv, _ := cmd.Flags().GetBool("csv")
	html, _ := cmd.Flags().GetBool("html")
	output, _ := cmd.Flags().GetString("output")
	dataflow, _ := cmd.Flags().GetBool("dataflow")
	quiet, _ := cmd.Flags().GetBool("quiet")
	singleline, _ := cmd.Flags().GetBool("singleline")
	context, _ := cmd.Flags().GetInt("context")

	// Get remaining args as target directories/files (default to current directory)
	targets := args
	if len(targets) == 0 {
		targets = []string{"."}
	}

	return quality.RunFlawfinder(quality.FlawfinderOptions{
		MinLevel:   minLevel,
		CSV:        csv,
		HTML:       html,
		Output:     output,
		Dataflow:   dataflow,
		Quiet:      quiet,
		Singleline: singleline,
		Context:    context,
		Targets:    targets,
	})
}
