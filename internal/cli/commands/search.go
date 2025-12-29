package commands

import (
	"github.com/ozacod/cpx/internal/build/bazel"
	"github.com/ozacod/cpx/internal/build/cmake"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/build/meson"
	"github.com/ozacod/cpx/internal/build/vcpkg"
	"github.com/ozacod/cpx/internal/cli/tui"
	"github.com/spf13/cobra"
)

func SearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search for libraries interactively",
		Long:  "Search for libraries using an interactive TUI. Select packages to add them to your project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, args)
		},
		Args: cobra.MaximumNArgs(1),
	}

	return cmd
}

func runSearch(_ *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	projectType := DetectProjectType()

	var builder build.BuildSystem

	switch projectType {
	case ProjectTypeBazel:
		builder = bazel.New()
	case ProjectTypeMeson:
		builder = meson.New()
	case ProjectTypeCMake:
		builder = cmake.New()
	case ProjectTypeVcpkg:
		builder = vcpkg.New()
	default:
		builder = vcpkg.New()
	}

	// Adapter for search
	searchFunc := func(q string) ([]tui.SearchResult, error) {
		deps, err := builder.SearchDependencies(q)
		if err != nil {
			return nil, err
		}
		var results []tui.SearchResult
		for _, dep := range deps {
			results = append(results, tui.SearchResult{
				Name:        dep.Name,
				Version:     dep.Version,
				Description: dep.Description,
			})
		}
		return results, nil
	}

	// Adapter for add
	addFunc := func(pkg string) error {
		return builder.AddDependency(pkg, "")
	}

	return tui.RunSearch(query, searchFunc, addFunc)
}
