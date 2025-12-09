package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/spf13/cobra"
)

var benchSetupVcpkgEnvFunc func() error

// BenchCmd creates the bench command
func BenchCmd(setupVcpkgEnv func() error) *cobra.Command {
	benchSetupVcpkgEnvFunc = setupVcpkgEnv

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Build and run benchmarks",
		Long:  "Build the project benchmarks and run them. Detects vcpkg/CMake or Bazel projects automatically.",
		Example: `  cpx bench            # Build + run all benchmarks
  cpx bench --verbose  # Show verbose output
  cpx bench --target //bench:myapp_bench  # Run specific benchmark (Bazel)`,
		RunE: runBenchCmd,
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show verbose build output")
	cmd.Flags().String("target", "", "Specific benchmark target to run (Bazel projects)")

	return cmd
}

func runBenchCmd(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	target, _ := cmd.Flags().GetString("target")

	// Detect project type
	projectType := DetectProjectType()

	if projectType == ProjectTypeBazel {
		return runBazelBench(verbose, target)
	}

	// Default: CMake/vcpkg
	return build.RunBenchmarks(verbose, benchSetupVcpkgEnvFunc)
}

func runBazelBench(verbose bool, target string) error {
	fmt.Printf("%sRunning Bazel benchmarks...%s\n", Cyan, Reset)

	// If no target specified, query for bench targets
	if target == "" {
		// Query for all cc_binary targets in bench directory
		queryCmd := exec.Command("bazel", "query", "kind(cc_binary, //bench:*)")
		output, err := queryCmd.Output()
		if err != nil {
			// Try to find bench target in BUILD.bazel
			target = findBenchTarget()
			if target == "" {
				return fmt.Errorf("no benchmark targets found in //bench")
			}
		} else {
			// Use first target from query
			targets := strings.TrimSpace(string(output))
			if targets == "" {
				return fmt.Errorf("no benchmark targets found in //bench")
			}
			// Take first target
			target = strings.Split(targets, "\n")[0]
		}
	}

	fmt.Printf("  Running: %s\n", target)

	bazelArgs := []string{"run", target}

	if verbose {
		bazelArgs = append(bazelArgs, "--verbose_failures")
	}

	benchCmd := exec.Command("bazel", bazelArgs...)
	benchCmd.Stdout = os.Stdout
	benchCmd.Stderr = os.Stderr

	if err := benchCmd.Run(); err != nil {
		return fmt.Errorf("bazel benchmark failed: %w", err)
	}

	fmt.Printf("%s✓ Benchmarks complete%s\n", Green, Reset)
	return nil
}

func findBenchTarget() string {
	// Try to read bench/BUILD.bazel to find a cc_binary target
	data, err := os.ReadFile("bench/BUILD.bazel")
	if err != nil {
		return ""
	}

	content := string(data)
	// Look for name = "xxx" pattern
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name = \"") {
			// Extract target name
			name := strings.TrimPrefix(line, "name = \"")
			name = strings.TrimSuffix(name, "\",")
			name = strings.TrimSuffix(name, "\"")
			return "//bench:" + name
		}
	}
	return ""
}
