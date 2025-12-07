package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// FindExecutables finds all executables in the build directory
func FindExecutables(buildDir string) ([]string, error) {
	var executables []string

	entries, err := os.ReadDir(buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read build directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()

		// Skip test executables and common non-executable files
		if strings.Contains(name, "_test") || strings.Contains(name, "_tests") ||
			strings.HasSuffix(name, ".a") || strings.HasSuffix(name, ".so") ||
			strings.HasSuffix(name, ".dylib") || strings.HasSuffix(name, ".dll") ||
			strings.HasSuffix(name, ".lib") || strings.HasSuffix(name, ".o") ||
			strings.HasSuffix(name, ".cmake") || strings.HasSuffix(name, ".ninja") ||
			strings.HasSuffix(name, ".make") || strings.HasSuffix(name, ".txt") {
			continue
		}

		// Check if it's executable
		if runtime.GOOS == "windows" {
			if strings.HasSuffix(name, ".exe") {
				executables = append(executables, filepath.Join(buildDir, name))
			}
		} else {
			if info.Mode()&0111 != 0 {
				executables = append(executables, filepath.Join(buildDir, name))
			}
		}
	}

	// Sort by name for consistent ordering
	sort.Strings(executables)

	return executables, nil
}

// RunProject builds and runs the project
func RunProject(release bool, target string, execArgs []string, verbose bool, setupVcpkgEnv func() error) error {
	// Set VCPKG_ROOT from cpx config if not already set
	if err := setupVcpkgEnv(); err != nil {
		return err
	}

	// Get project name from CMakeLists.txt (optional, for display only)
	projectName := GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	buildType, _ := DetermineBuildType(release, "")

	optLabel := "default (CMake)"

	fmt.Printf("\n%s▸ Build%s %s %s(%s)%s %s[opt: %s]%s\n",
		colorCyan, colorReset, projectName, colorGray, buildType, colorReset,
		colorGray, optLabel, colorReset)

	// Configure CMake if needed
	buildDir := "build"
	needsConfigure := false
	if _, err := os.Stat(filepath.Join(buildDir, "CMakeCache.txt")); os.IsNotExist(err) {
		needsConfigure = true
	}

	// Determine total steps
	totalSteps := 1
	currentStep := 0
	if needsConfigure {
		totalSteps = 2
	}

	if needsConfigure {
		currentStep++
		if verbose {
			fmt.Printf("%s  • Configuring CMake%s\n", colorCyan, colorReset)
		} else {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configuring...", colorCyan, currentStep, totalSteps, colorReset)
		}

		// Check if CMakePresets.json exists, use preset if available
		if _, err := os.Stat("CMakePresets.json"); err == nil {
			// Use "default" preset (VCPKG_ROOT is now set from config)
			cmd := exec.Command("cmake", "--preset=default")
			cmd.Env = os.Environ()
			if err := runCMakeConfigure(cmd, verbose); err != nil {
				fmt.Println()
				return fmt.Errorf("cmake configure failed (preset 'default'): %w", err)
			}
		} else {
			// Fallback to traditional cmake configure
			cmd := exec.Command("cmake", "-B", buildDir, "-DCMAKE_BUILD_TYPE="+buildType)
			if err := runCMakeConfigure(cmd, verbose); err != nil {
				fmt.Println()
				return fmt.Errorf("cmake configure failed: %w", err)
			}
		}

		if !verbose {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configured ✓\n", colorCyan, currentStep, totalSteps, colorReset)
		}
	}

	// Build specific target if provided
	buildStart := time.Now()
	buildArgs := []string{"--build", buildDir, "--config", buildType}
	if target != "" {
		buildArgs = append(buildArgs, "--target", target)
	}

	currentStep++
	if err := runCMakeBuild(buildArgs, verbose, currentStep, totalSteps); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Find executable to run
	var execPath string

	// If target specified, look for that specific executable
	if target != "" {
		targetName := target
		if runtime.GOOS == "windows" && !strings.HasSuffix(targetName, ".exe") {
			targetName += ".exe"
		}
		execPath = filepath.Join(buildDir, targetName)
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			return fmt.Errorf("target executable '%s' not found in %s", target, buildDir)
		}
	} else {
		// Look for project name executable first
		execName := projectName
		if runtime.GOOS == "windows" {
			execName += ".exe"
		}

		execPath = filepath.Join(buildDir, execName)
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			// Find all executables
			executables, err := FindExecutables(buildDir)
			if err != nil {
				return err
			}

			if len(executables) == 0 {
				return fmt.Errorf("no executable found in %s. Make sure the project builds an executable", buildDir)
			}

			if len(executables) == 1 {
				execPath = executables[0]
			} else {
				// Multiple executables found, list them
				fmt.Printf("%s Multiple executables found:%s\n", colorGray, colorReset)
				for i, executable := range executables {
					fmt.Printf("  [%d] %s\n", i+1, filepath.Base(executable))
				}
				fmt.Printf("\nUse --target <name> to specify which one to run\n")
				// Run the first one by default
				execPath = executables[0]
				fmt.Printf("%s Running first: %s%s\n", "\033[33m", filepath.Base(execPath), "\033[0m")
			}
		}
	}

	fmt.Printf("%s  ✔ Build complete%s %s[%s]%s\n", colorGreen, colorReset, colorGray, time.Since(buildStart).Round(10*time.Millisecond), colorReset)
	fmt.Printf("%s  ▶ Run%s %s%s%s\n\n", colorCyan, colorReset, colorGreen, filepath.Base(execPath), colorReset)
	fmt.Println(strings.Repeat("─", 40))

	runCmd := exec.Command(execPath, execArgs...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin
	return runCmd.Run()
}
