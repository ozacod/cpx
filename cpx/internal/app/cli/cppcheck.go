package cli

import (
	"github.com/ozacod/cpx/internal/pkg/quality"
	"github.com/spf13/cobra"
)

func CppcheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cppcheck",
		Short: "Run Cppcheck static analysis for C/C++",
		Long:  "Run Cppcheck static analysis for C/C++. Performs static code analysis on C/C++ code.",
		RunE:  runCppcheck,
		Args:  cobra.ArbitraryArgs,
	}

	cmd.Flags().String("enable", "all", "Enable checks (all, style, performance, portability, information, unusedFunction, missingInclude)")
	cmd.Flags().String("output", "", "Output file path (for XML/CSV output)")
	cmd.Flags().Bool("xml", false, "Output results in XML format")
	cmd.Flags().Bool("csv", false, "Output results in CSV format")
	cmd.Flags().Bool("quiet", false, "Quiet mode (suppress progress messages)")
	cmd.Flags().Bool("force", false, "Force checking of all configurations")
	cmd.Flags().Bool("inline-suppr", false, "Enable inline suppressions")
	cmd.Flags().String("platform", "", "Target platform (unix32, unix64, win32A, win32W, win64, avr8, etc.)")
	cmd.Flags().String("std", "", "C/C++ standard (c89, c99, c11, c++03, c++11, c++14, c++17, c++20)")

	return cmd
}

func runCppcheck(cmd *cobra.Command, args []string) error {
	enable, _ := cmd.Flags().GetString("enable")
	output, _ := cmd.Flags().GetString("output")
	xml, _ := cmd.Flags().GetBool("xml")
	csv, _ := cmd.Flags().GetBool("csv")
	quiet, _ := cmd.Flags().GetBool("quiet")
	force, _ := cmd.Flags().GetBool("force")
	inlineSuppr, _ := cmd.Flags().GetBool("inline-suppr")
	platform, _ := cmd.Flags().GetString("platform")
	std, _ := cmd.Flags().GetString("std")

	// Get remaining args as target directories/files (default to current directory)
	targets := args
	if len(targets) == 0 {
		targets = []string{"."}
	}

	return quality.RunCppcheck(quality.CppcheckOptions{
		Enable:      enable,
		Output:      output,
		XML:         xml,
		CSV:         csv,
		Quiet:       quiet,
		Force:       force,
		InlineSuppr: inlineSuppr,
		Platform:    platform,
		Std:         std,
		Targets:     targets,
	})
}
