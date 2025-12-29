package quality

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ozacod/cpx/internal/utils/colors"
	"github.com/ozacod/cpx/internal/utils/git"
)

// CppcheckOptions defines the configuration for RunCppcheck
type CppcheckOptions struct {
	Enable      string
	Output      string
	XML         bool
	CSV         bool
	Quiet       bool
	Force       bool
	InlineSuppr bool
	Platform    string
	Std         string
	Targets     []string
}

func RunCppcheck(opts CppcheckOptions) error {
	// Check if cppcheck is available
	if _, err := exec.LookPath("cppcheck"); err != nil {
		return fmt.Errorf("cppcheck not found. Please install it first:\n  brew install cppcheck\n  or\n  apt-get install cppcheck (Debian/Ubuntu)\n  or\n  Download from https://cppcheck.sourcecpx.io/")
	}

	fmt.Printf("%s Running Cppcheck analysis...%s\n", colors.Cyan, colors.Reset)

	// Filter targets to only include git-tracked files (respect .gitignore)
	filteredTargets, err := git.FilterGitTrackedFiles(opts.Targets)
	if err != nil {
		// If git is not available or not in a git repo, use original targets
		fmt.Printf("%s Warning: Not in a git repository or git not available. Scanning all files.%s\n", colors.Yellow, colors.Reset)
		filteredTargets = opts.Targets
	} else if len(filteredTargets) == 0 {
		return fmt.Errorf("no git-tracked C/C++ files found to scan")
	}

	// Build cppcheck command
	var cppcheckArgs []string

	// Enable checks
	if opts.Enable != "" {
		cppcheckArgs = append(cppcheckArgs, "--enable="+opts.Enable)
	}

	// Output format
	if opts.XML {
		cppcheckArgs = append(cppcheckArgs, "--xml")
	} else if opts.CSV {
		// Cppcheck uses --template for CSV format, not --csv
		cppcheckArgs = append(cppcheckArgs, "--template={file},{line},{severity},{id},{message}")
	}

	// Output file
	if opts.Output != "" {
		cppcheckArgs = append(cppcheckArgs, "--output-file="+opts.Output)
		fmt.Printf("%s Writing output to: %s%s\n", colors.Cyan, opts.Output, colors.Reset)
	}

	// Quiet mode
	if opts.Quiet {
		cppcheckArgs = append(cppcheckArgs, "--quiet")
	}

	// Force checking all configurations
	if opts.Force {
		cppcheckArgs = append(cppcheckArgs, "--force")
	}

	// Inline suppressions
	if opts.InlineSuppr {
		cppcheckArgs = append(cppcheckArgs, "--inline-suppr")
	}

	// Platform
	if opts.Platform != "" {
		cppcheckArgs = append(cppcheckArgs, "--platform="+opts.Platform)
	}

	// C/C++ standard
	if opts.Std != "" {
		cppcheckArgs = append(cppcheckArgs, "--std="+opts.Std)
	}

	// Add exclusions for build system directories and external dependencies
	// to prevent scanning third-party code
	excludeDirs := []string{
		"build",       // CMake build dir
		"builddir",    // Meson build dir
		"subprojects", // Meson subprojects
		"external",    // Bazel external
		".bazel",      // Bazel cache
		".cache",      // vcpkg cache
		"bazel-bin",   // Bazel output
		"bazel-out",   // Bazel output
		"bazel-testlogs",
		"out",
		"bin",
		".vcpkg",
	}

	// Exclude any directory starting with "bazel-" (project-specific bazel dirs)
	for _, dir := range excludeDirs {
		cppcheckArgs = append(cppcheckArgs, "-i"+dir)
	}

	// Add target files
	cppcheckArgs = append(cppcheckArgs, filteredTargets...)

	cmd := exec.Command("cppcheck", cppcheckArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Cppcheck returns non-zero on findings, which is normal
		if opts.Output != "" {
			fmt.Printf("%s  Cppcheck found potential issues (saved to %s)%s\n", colors.Yellow, opts.Output, colors.Reset)
		} else {
			fmt.Printf("%s  Cppcheck found potential issues%s\n", colors.Yellow, colors.Reset)
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
