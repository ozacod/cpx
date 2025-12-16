package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/ozacod/cpx/internal/pkg/vcpkg"
	"github.com/spf13/cobra"
)

// RunCmd creates the run command
func RunCmd(client *vcpkg.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Build and run the project",
		Long: `Build the project and run the executable. Automatically detects project type:
  - vcpkg/CMake projects: Builds with CMake and runs the binary
  - Bazel projects: Uses bazel run

Arguments after -- are passed to the binary.`,
		Example: `  cpx run                 # Debug build by default
  cpx run --release        # Release build, then run
  cpx run --asan           # Run with AddressSanitizer
  cpx run --target app -- --flag value`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, args, client)
		},
	}

	cmd.Flags().Bool("release", false, "Build in release mode (-O2). Default is debug")
	cmd.Flags().String("toolchain", "", "Toolchain to run in Docker (from cpx-ci.yaml)")
	cmd.Flags().StringP("opt", "O", "", "Override optimization level: 0,1,2,3,s,fast")
	cmd.Flags().Bool("verbose", false, "Show full build output")
	// Sanitizer flags
	cmd.Flags().Bool("asan", false, "Run with AddressSanitizer")
	cmd.Flags().Bool("tsan", false, "Run with ThreadSanitizer")
	cmd.Flags().Bool("msan", false, "Run with MemorySanitizer")
	cmd.Flags().Bool("ubsan", false, "Run with UndefinedBehaviorSanitizer")

	return cmd
}

func runRun(cmd *cobra.Command, args []string, client *vcpkg.Client) error {
	release, _ := cmd.Flags().GetBool("release")
	toolchain, _ := cmd.Flags().GetString("toolchain")
	optLevel, _ := cmd.Flags().GetString("opt")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// --toolchain is for CI builds (Docker)
	if toolchain != "" {
		return runToolchainBuild(toolchain, false, true, false, false) // executeAfterBuild=true, runTests=false, runBenchmarks=false
	}

	// Parse sanitizer flags
	asan, _ := cmd.Flags().GetBool("asan")
	tsan, _ := cmd.Flags().GetBool("tsan")
	msan, _ := cmd.Flags().GetBool("msan")
	ubsan, _ := cmd.Flags().GetBool("ubsan")

	// Validate only one sanitizer is used
	sanitizer := ""
	sanitizerCount := 0
	if asan {
		sanitizer = "asan"
		sanitizerCount++
	}
	if tsan {
		sanitizer = "tsan"
		sanitizerCount++
	}
	if msan {
		sanitizer = "msan"
		sanitizerCount++
	}
	if ubsan {
		sanitizer = "ubsan"
		sanitizerCount++
	}
	if sanitizerCount > 1 {
		return fmt.Errorf("only one sanitizer can be used at a time (got %d)", sanitizerCount)
	}

	projectType := DetectProjectType()

	// Check for missing build tools and warn the user
	WarnMissingBuildTools(projectType)

	switch projectType {
	case ProjectTypeBazel:
		return runBazelRun(release, "", args, verbose, optLevel, sanitizer)
	case ProjectTypeMeson:
		return runMesonRun(release, "", args, verbose, optLevel, sanitizer)
	case ProjectTypeVcpkg:
		return build.RunProject(release, "", args, verbose, optLevel, sanitizer, client)
	default:
		// Fall back to CMake run even without vcpkg.json
		return build.RunProject(release, "", args, verbose, optLevel, sanitizer, client)
	}
}

func runBazelRun(release bool, target string, args []string, verbose bool, optLevel string, sanitizer string) error {
	// Build bazel run args
	bazelArgs := []string{"run"}

	// Handle optimization level
	switch optLevel {
	case "0":
		bazelArgs = append(bazelArgs, "--copt=-O0", "-c", "dbg")
	case "1":
		bazelArgs = append(bazelArgs, "--copt=-O1", "-c", "opt")
	case "2":
		bazelArgs = append(bazelArgs, "--copt=-O2", "-c", "opt")
	case "3":
		bazelArgs = append(bazelArgs, "--copt=-O3", "-c", "opt")
	case "s":
		bazelArgs = append(bazelArgs, "--copt=-Os", "-c", "opt")
	case "fast":
		bazelArgs = append(bazelArgs, "--copt=-Ofast", "-c", "opt")
	default:
		if release {
			bazelArgs = append(bazelArgs, "--config=release")
		} else {
			bazelArgs = append(bazelArgs, "--config=debug")
		}
	}

	// Add sanitizer flags
	if sanitizer != "" {
		switch sanitizer {
		case "asan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=address", "--copt=-fno-omit-frame-pointer",
				"--linkopt=-fsanitize=address")
		case "tsan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=thread", "--linkopt=-fsanitize=thread")
		case "msan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=memory", "--copt=-fno-omit-frame-pointer",
				"--linkopt=-fsanitize=memory")
		case "ubsan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=undefined", "--linkopt=-fsanitize=undefined")
		}
	}

	// Add target or try to find one
	if target != "" {
		if !strings.HasPrefix(target, "//") && !strings.HasPrefix(target, ":") {
			target = "//:" + target
		}
		bazelArgs = append(bazelArgs, target)
	} else {
		// Try to find the main target from BUILD.bazel
		mainTarget, err := findBazelMainTarget()
		if err != nil {
			return fmt.Errorf("no target specified and could not find main target: %w\n  hint: use --target to specify the target", err)
		}
		bazelArgs = append(bazelArgs, mainTarget)
	}

	// Add -- and user args if present
	if len(args) > 0 {
		bazelArgs = append(bazelArgs, "--")
		bazelArgs = append(bazelArgs, args...)
	}

	fmt.Printf("%sRunning with Bazel...%s\n", Cyan, Reset)
	if verbose {
		fmt.Printf("  Running: bazel %v\n", bazelArgs)
	} else {
		// Use hidden symlinks (.bazel-bin, .bazel-out, etc.)
		bazelArgs = append(bazelArgs, "--noshow_progress", "--symlink_prefix=.bazel-")
	}

	runCmd := execCommand("bazel", bazelArgs...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	return runCmd.Run()
}

func runMesonRun(release bool, target string, args []string, verbose bool, optLevel string, sanitizer string) error {
	// Ensure project is built first
	if err := runMesonBuild(release, target, false, verbose, optLevel, sanitizer); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Find executable to run
	var exePath string
	if target != "" {
		// Try in src/ subdirectory first, then builddir root
		srcPath := filepath.Join("builddir", "src", target)
		if _, err := os.Stat(srcPath); err == nil {
			exePath = srcPath
		} else {
			exePath = filepath.Join("builddir", target)
		}
	} else {
		// Look for executables in builddir/src/ first (Meson puts main exe there)
		searchDirs := []string{filepath.Join("builddir", "src"), "builddir"}
		for _, dir := range searchDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					continue
				}
				// Check if executable (not test, lib, or dylib)
				name := entry.Name()
				if info.Mode()&0111 != 0 &&
					!strings.HasSuffix(name, "_test") &&
					!strings.HasSuffix(name, "_bench") &&
					!strings.HasSuffix(name, ".a") &&
					!strings.HasSuffix(name, ".so") &&
					!strings.HasSuffix(name, ".dylib") {
					exePath = filepath.Join(dir, name)
					break
				}
			}
			if exePath != "" {
				break
			}
		}
	}

	if exePath == "" {
		return fmt.Errorf("no executable found in builddir\n  hint: use --target to specify the executable")
	}

	fmt.Printf("%sRunning %s...%s\n", Cyan, exePath, Reset)
	runCmd := execCommand(exePath, args...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	return runCmd.Run()
}

// findBazelMainTarget tries to find a cc_binary target in BUILD.bazel
func findBazelMainTarget() (string, error) {
	// Read BUILD.bazel
	content, err := os.ReadFile("BUILD.bazel")
	if err != nil {
		return "", fmt.Errorf("could not read BUILD.bazel: %w", err)
	}

	// Look for cc_binary declarations
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name = \"") {
			// Extract target name
			name := strings.TrimPrefix(line, "name = \"")
			name = strings.TrimSuffix(name, "\",")
			name = strings.TrimSuffix(name, "\"")
			// Skip library targets (usually end with _lib)
			if !strings.HasSuffix(name, "_lib") && !strings.HasSuffix(name, "_test") {
				return "//:" + name, nil
			}
		}
	}

	// Fallback: use project directory name
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	projectName := filepath.Base(cwd)
	return "//:" + projectName, nil
}
