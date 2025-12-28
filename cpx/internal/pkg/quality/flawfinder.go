package quality

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ozacod/cpx/internal/pkg/utils/colors"
	"github.com/ozacod/cpx/internal/pkg/utils/git"
)

// FlawfinderOptions defines the configuration for RunFlawfinder
type FlawfinderOptions struct {
	MinLevel   int
	CSV        bool
	HTML       bool
	Output     string
	Dataflow   bool
	Quiet      bool
	Singleline bool
	Context    int
	Targets    []string
}

func RunFlawfinder(opts FlawfinderOptions) error {
	// Check if flawfinder is available
	if _, err := exec.LookPath("flawfinder"); err != nil {
		return fmt.Errorf("flawfinder not found. Please install it first:\n  pip install flawfinder\n  or\n  brew install flawfinder\n  or\n  apt-get install flawfinder (Debian/Ubuntu)")
	}

	// Validate output file for HTML/CSV
	if (opts.HTML || opts.CSV) && opts.Output == "" {
		return fmt.Errorf("--output file is required when using --html or --csv flags")
	}

	fmt.Printf("%s Running Flawfinder analysis...%s\n", colors.Cyan, colors.Reset)

	// Filter targets to only include git-tracked files (respect .gitignore)
	filteredTargets, err := git.FilterGitTrackedFiles(opts.Targets)
	if err != nil {
		// If git is not available or not in a git repo, use original targets
		fmt.Printf("%s Warning: Not in a git repository or git not available. Scanning all files.%s\n", colors.Yellow, colors.Reset)
		filteredTargets = opts.Targets
	} else if len(filteredTargets) == 0 {
		return fmt.Errorf("no git-tracked C/C++ files found to scan")
	}

	// Build flawfinder command
	var flawfinderArgs []string

	// Add min level
	if opts.MinLevel >= 0 && opts.MinLevel <= 5 {
		flawfinderArgs = append(flawfinderArgs, "-m", fmt.Sprintf("%d", opts.MinLevel))
	}

	// Output format
	if opts.CSV {
		flawfinderArgs = append(flawfinderArgs, "-C")
	} else if opts.HTML {
		flawfinderArgs = append(flawfinderArgs, "-H")
	}

	// Dataflow analysis
	if opts.Dataflow {
		flawfinderArgs = append(flawfinderArgs, "-D")
	}

	// Quiet mode
	if opts.Quiet {
		flawfinderArgs = append(flawfinderArgs, "--quiet")
	}

	// Single line output
	if opts.Singleline {
		flawfinderArgs = append(flawfinderArgs, "--singleline")
	}

	// Context lines
	if opts.Context > 0 {
		flawfinderArgs = append(flawfinderArgs, "-c", fmt.Sprintf("%d", opts.Context))
	}

	// Add filtered target files
	flawfinderArgs = append(flawfinderArgs, filteredTargets...)

	cmd := exec.Command("flawfinder", flawfinderArgs...)

	// Handle output file for HTML/CSV
	if opts.Output != "" {
		file, err := os.Create(opts.Output)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		cmd.Stdout = file
		fmt.Printf("%s Writing output to: %s%s\n", colors.Cyan, opts.Output, colors.Reset)
	} else {
		cmd.Stdout = os.Stdout
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Flawfinder returns non-zero on findings, which is normal
		if opts.Output != "" {
			fmt.Printf("%s  Flawfinder found potential issues (saved to %s)%s\n", colors.Yellow, opts.Output, colors.Reset)
		} else {
			fmt.Printf("%s  Flawfinder found potential issues%s\n", colors.Yellow, colors.Reset)
		}
		return nil
	}

	if opts.Output != "" {
		fmt.Printf("%s Analysis complete! Report saved to: %s%s\n", colors.Green, opts.Output, colors.Reset)
	} else {
		fmt.Printf("%s No issues found!%s\n", colors.Green, colors.Reset)
	}
	return nil
}
