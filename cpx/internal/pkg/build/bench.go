package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ozacod/cpx/internal/pkg/vcpkg"
)

// RunBenchmarks builds and runs the project benchmarks
func RunBenchmarks(verbose bool, vcpkgClient *vcpkg.Client) error {
	// Set VCPKG_ROOT from cpx config if not already set
	if err := vcpkgClient.SetupEnv(); err != nil {
		return err
	}

	projectName := GetProjectNameFromCMakeLists()
	if projectName == "" {
		return fmt.Errorf("failed to get project name from CMakeLists.txt")
	}
	fmt.Printf("%s Running benchmarks for '%s'...%s\n", "\033[36m", projectName, "\033[0m")

	// Default to release for benchmarks (benchmarks should be optimized)
	// Use .cache/native/bench for building benchmarks (separate from normal builds)
	buildDir := filepath.Join(".cache", "native", "bench")
	benchTarget := projectName + "_bench"

	// Check if configure is needed
	needsConfigure := false
	if _, err := os.Stat(filepath.Join(buildDir, "CMakeCache.txt")); os.IsNotExist(err) {
		needsConfigure = true
	}

	// Determine total steps: configure (optional) + build + run
	totalSteps := 2 // build + run
	if needsConfigure {
		totalSteps = 3 // configure + build + run
	}
	currentStep := 0

	// Configure CMake if needed
	if needsConfigure {
		currentStep++
		if verbose {
			fmt.Printf("%s  Configuring CMake (with benchmarks enabled)...%s\n", "\033[36m", "\033[0m")
		} else {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configuring...", colorCyan, currentStep, totalSteps, colorReset)
		}

		// Determine absolute path for shared vcpkg_installed directory
		cwd, _ := os.Getwd()
		vcpkgInstalledDir := filepath.Join(cwd, ".cache", "native", "vcpkg_installed")
		vcpkgInstallArg := "-DVCPKG_INSTALLED_DIR=" + vcpkgInstalledDir

		// Enable benchmarks with Release build type for optimal performance
		enableBenchArg := "-DENABLE_BENCHMARKS=ON"
		buildTypeArg := "-DCMAKE_BUILD_TYPE=Release"

		// Check if CMakePresets.json exists, use preset if available
		if _, err := os.Stat("CMakePresets.json"); err == nil {
			cmd := exec.Command("cmake", "--preset=default", "-B", buildDir, vcpkgInstallArg, enableBenchArg, buildTypeArg)
			cmd.Env = os.Environ()
			if err := runCMakeConfigure(cmd, verbose); err != nil {
				fmt.Println()
				return fmt.Errorf("cmake configure failed (preset 'default'): %w", err)
			}
		} else {
			cmd := exec.Command("cmake", "-B", buildDir, vcpkgInstallArg, enableBenchArg, buildTypeArg)
			if err := runCMakeConfigure(cmd, verbose); err != nil {
				fmt.Println()
				return fmt.Errorf("cmake configure failed: %w", err)
			}
		}

		if !verbose {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configured ✓\n", colorCyan, currentStep, totalSteps, colorReset)
		}
	}

	// Build benchmarks
	currentStep++
	buildArgs := []string{"--build", buildDir, "--target", benchTarget}
	if err := runCMakeBuild(buildArgs, verbose, currentStep, totalSteps); err != nil {
		return fmt.Errorf("failed to build benchmarks: %w", err)
	}

	// Run benchmarks
	currentStep++
	if !verbose {
		fmt.Printf("%s[%d/%d]%s Running benchmarks...\n", colorCyan, currentStep, totalSteps, colorReset)
	} else {
		fmt.Printf("%s Running benchmarks...%s\n", "\033[36m", "\033[0m")
	}

	// Find the benchmark executable
	// Try common locations
	possiblePaths := []string{
		filepath.Join(buildDir, "bench", benchTarget),
		filepath.Join(buildDir, benchTarget),
	}

	var benchPath string
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			benchPath = p
			break
		}
	}

	if benchPath == "" {
		return fmt.Errorf("benchmark executable not found. Tried: %v", possiblePaths)
	}

	benchCmd := exec.Command(benchPath)
	benchCmd.Stdout = os.Stdout
	benchCmd.Stderr = os.Stderr

	fmt.Println() // Add blank line before benchmark output
	if err := benchCmd.Run(); err != nil {
		return fmt.Errorf("benchmarks failed: %w", err)
	}

	fmt.Printf("\n%s✓ Benchmarks completed!%s\n", "\033[32m", "\033[0m")
	return nil
}
