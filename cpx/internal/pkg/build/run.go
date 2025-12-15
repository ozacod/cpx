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

	"github.com/ozacod/cpx/internal/pkg/vcpkg"
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
func RunProject(release bool, target string, execArgs []string, verbose bool, optLevel string, sanitizer string, vcpkgClient *vcpkg.Client) error {
	// Set VCPKG_ROOT from cpx config if not already set
	if err := vcpkgClient.SetupEnv(); err != nil {
		return err
	}

	// Get project name from CMakeLists.txt (optional, for display only)
	projectName := GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	buildType, cxxFlags := DetermineBuildType(release, optLevel)

	// Add sanitizer flags
	sanCFlags, sanLFlags := GetSanitizerFlags(sanitizer)
	cxxFlags += sanCFlags
	linkerFlags := sanLFlags

	optLabel := "default (-O0)"
	if release {
		optLabel = "-O2 (Release)"
	}
	if optLevel != "" {
		optLabel = "-O" + optLevel
	}
	if sanitizer != "" {
		optLabel += "+" + sanitizer
	}

	fmt.Printf("\n%s▸ Build%s %s %s(%s)%s %s[opt: %s]%s\n",
		colorCyan, colorReset, projectName, colorGray, buildType, colorReset,
		colorGray, optLabel, colorReset)

	// Configure CMake if needed
	outDirName := "debug"
	if optLevel != "" {
		outDirName = "O" + optLevel
	} else if release {
		outDirName = "release"
	}
	// Append sanitizer suffix
	if sanitizer != "" {
		outDirName += "-" + sanitizer
	}
	cacheBuildDir := filepath.Join(".cache", "native", outDirName)
	finalBuildDir := filepath.Join(".bin", "native", outDirName)
	needsConfigure := false
	if _, err := os.Stat(filepath.Join(cacheBuildDir, "CMakeCache.txt")); os.IsNotExist(err) {
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

		// Determine absolute path for shared vcpkg_installed directory
		cwd, _ := os.Getwd()
		vcpkgInstalledDir := filepath.Join(cwd, ".cache", "native", "vcpkg_installed")
		vcpkgInstallArg := "-DVCPKG_INSTALLED_DIR=" + vcpkgInstalledDir

		// Check if CMakePresets.json exists, use preset if available
		if _, err := os.Stat("CMakePresets.json"); err == nil {
			// Use "default" preset (VCPKG_ROOT is now set from config)
			cmdArgs := []string{"--preset=default", "-B", cacheBuildDir, vcpkgInstallArg}
			if cxxFlags != "" {
				cmdArgs = append(cmdArgs, "-DCMAKE_CXX_FLAGS="+cxxFlags, "-DCMAKE_C_FLAGS="+cxxFlags)
			}
			if linkerFlags != "" {
				cmdArgs = append(cmdArgs, "-DCMAKE_EXE_LINKER_FLAGS="+linkerFlags, "-DCMAKE_SHARED_LINKER_FLAGS="+linkerFlags)
			}
			cmd := exec.Command("cmake", cmdArgs...)
			cmd.Env = os.Environ()
			if err := runCMakeConfigure(cmd, verbose); err != nil {
				fmt.Println()
				return fmt.Errorf("cmake configure failed (preset 'default'): %w", err)
			}
		} else {
			// Fallback to traditional cmake configure
			cmdArgs := []string{"-B", cacheBuildDir, "-DCMAKE_BUILD_TYPE=" + buildType, vcpkgInstallArg}
			if cxxFlags != "" {
				cmdArgs = append(cmdArgs, "-DCMAKE_CXX_FLAGS="+cxxFlags, "-DCMAKE_C_FLAGS="+cxxFlags)
			}
			if linkerFlags != "" {
				cmdArgs = append(cmdArgs, "-DCMAKE_EXE_LINKER_FLAGS="+linkerFlags, "-DCMAKE_SHARED_LINKER_FLAGS="+linkerFlags)
			}
			cmd := exec.Command("cmake", cmdArgs...)
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
	// Build in .cache directory
	buildArgs := []string{"--build", cacheBuildDir, "--config", buildType}
	if target != "" {
		buildArgs = append(buildArgs, "--target", target)
	}

	currentStep++
	if err := runCMakeBuild(buildArgs, verbose, currentStep, totalSteps); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Copy artifacts to final build directory
	if err := os.MkdirAll(finalBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create final build dir: %w", err)
	}

	executables, err := FindExecutables(cacheBuildDir)
	if err == nil {
		for _, exe := range executables {
			dest := filepath.Join(finalBuildDir, filepath.Base(exe))
			_ = copyAndSign(exe, dest)
		}
	}

	// Find executable to run (in finalBuildDir)
	var execPath string

	// If target specified, look for that specific executable
	if target != "" {
		targetName := target
		if runtime.GOOS == "windows" && !strings.HasSuffix(targetName, ".exe") {
			targetName += ".exe"
		}
		execPath = filepath.Join(finalBuildDir, targetName)
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			return fmt.Errorf("target executable '%s' not found in %s", target, finalBuildDir)
		}
	} else {
		// Look for project name executable first
		execName := projectName
		if runtime.GOOS == "windows" {
			execName += ".exe"
		}

		execPath = filepath.Join(finalBuildDir, execName)
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			// Find all executables
			executables, err := FindExecutables(finalBuildDir)
			if err != nil {
				return err
			}

			if len(executables) == 0 {
				return fmt.Errorf("no executable found in %s. Make sure the project builds an executable", finalBuildDir)
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
