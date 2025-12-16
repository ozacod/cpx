package quality

import (
	"fmt"
	"os"
	"os/exec"
)

// RunCppcheck runs Cppcheck static analysis for C/C++
func RunCppcheck(enable, output string, xml, csv, quiet, force, inlineSuppr bool, platform, std string, targets []string) error {
	// Check if cppcheck is available
	if _, err := exec.LookPath("cppcheck"); err != nil {
		return fmt.Errorf("cppcheck not found. Please install it first:\n  brew install cppcheck\n  or\n  apt-get install cppcheck (Debian/Ubuntu)\n  or\n  Download from https://cppcheck.sourcecpx.io/")
	}

	fmt.Printf("%s Running Cppcheck analysis...%s\n", Cyan, Reset)

	// Filter targets to only include git-tracked files (respect .gitignore)
	filteredTargets, err := FilterGitTrackedFiles(targets)
	if err != nil {
		// If git is not available or not in a git repo, use original targets
		fmt.Printf("%s Warning: Not in a git repository or git not available. Scanning all files.%s\n", Yellow, Reset)
		filteredTargets = targets
	} else if len(filteredTargets) == 0 {
		return fmt.Errorf("no git-tracked C/C++ files found to scan")
	}

	// Build cppcheck command
	cppcheckArgs := []string{}

	// Enable checks
	if enable != "" {
		cppcheckArgs = append(cppcheckArgs, "--enable="+enable)
	}

	// Output format
	if xml {
		cppcheckArgs = append(cppcheckArgs, "--xml")
	} else if csv {
		// Cppcheck uses --template for CSV format, not --csv
		cppcheckArgs = append(cppcheckArgs, "--template={file},{line},{severity},{id},{message}")
	}

	// Output file
	if output != "" {
		cppcheckArgs = append(cppcheckArgs, "--output-file="+output)
		fmt.Printf("%s Writing output to: %s%s\n", Cyan, output, Reset)
	}

	// Quiet mode
	if quiet {
		cppcheckArgs = append(cppcheckArgs, "--quiet")
	}

	// Force checking all configurations
	if force {
		cppcheckArgs = append(cppcheckArgs, "--force")
	}

	// Inline suppressions
	if inlineSuppr {
		cppcheckArgs = append(cppcheckArgs, "--inline-suppr")
	}

	// Platform
	if platform != "" {
		cppcheckArgs = append(cppcheckArgs, "--platform="+platform)
	}

	// C/C++ standard
	if std != "" {
		cppcheckArgs = append(cppcheckArgs, "--std="+std)
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
		if output != "" {
			fmt.Printf("%s  Cppcheck found potential issues (saved to %s)%s\n", Yellow, output, Reset)
		} else {
			fmt.Printf("%s  Cppcheck found potential issues%s\n", Yellow, Reset)
		}
		return nil
	}

	if output != "" {
		fmt.Printf("%s Analysis complete! Report saved to: %s%s\n", Green, output, Reset)
	} else {
		fmt.Printf("%s No issues found!%s\n", Green, Reset)
	}
	return nil
}
