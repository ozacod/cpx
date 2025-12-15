package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ozacod/cpx/internal/pkg/vcpkg"
)

// RunTests runs the project tests
func RunTests(verbose bool, filter string, vcpkgClient *vcpkg.Client) error {
	// Set VCPKG_ROOT from cpx config if not already set
	if err := vcpkgClient.SetupEnv(); err != nil {
		return err
	}

	projectName := GetProjectNameFromCMakeLists()
	if projectName == "" {
		return fmt.Errorf("failed to get project name from CMakeLists.txt")
	}
	fmt.Printf("%s Running tests for '%s'...%s\n", "\033[36m", projectName, "\033[0m")

	// Default to debug for tests if no config specified
	// Use .cache/native/test for building tests (separate from normal builds)
	buildDir := filepath.Join(".cache", "native", "test")

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
			fmt.Printf("%s  Configuring CMake (with testing enabled)...%s\n", "\033[36m", "\033[0m")
		} else {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configuring...", colorCyan, currentStep, totalSteps, colorReset)
		}

		// Determine absolute path for shared vcpkg_installed directory
		cwd, _ := os.Getwd()
		vcpkgInstalledDir := filepath.Join(cwd, ".cache", "native", "vcpkg_installed")
		vcpkgInstallArg := "-DVCPKG_INSTALLED_DIR=" + vcpkgInstalledDir

		// Enable testing
		enableTestingArg := "-DENABLE_TESTING=ON"

		// Check if CMakePresets.json exists, use preset if available
		if _, err := os.Stat("CMakePresets.json"); err == nil {
			// Use "default" preset (VCPKG_ROOT is now set from config)
			cmd := exec.Command("cmake", "--preset=default", "-B", buildDir, vcpkgInstallArg, enableTestingArg)
			cmd.Env = os.Environ()
			if err := runCMakeConfigure(cmd, verbose); err != nil {
				fmt.Println()
				return fmt.Errorf("cmake configure failed (preset 'default'): %w", err)
			}
		} else {
			// Fallback to traditional cmake configure
			cmd := exec.Command("cmake", "-B", buildDir, vcpkgInstallArg, enableTestingArg)
			if err := runCMakeConfigure(cmd, verbose); err != nil {
				fmt.Println()
				return fmt.Errorf("cmake configure failed: %w", err)
			}
		}

		if !verbose {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configured ✓\n", colorCyan, currentStep, totalSteps, colorReset)
		}
	}

	// Build tests
	currentStep++
	buildArgs := []string{"--build", buildDir, "--target", projectName + "_tests"}
	if err := runCMakeBuild(buildArgs, verbose, currentStep, totalSteps); err != nil {
		return fmt.Errorf("failed to build tests: %w", err)
	}

	// Run tests with CTest
	currentStep++
	if !verbose {
		fmt.Printf("%s[%d/%d]%s Running tests...\n", colorCyan, currentStep, totalSteps, colorReset)
	} else {
		fmt.Printf("%s Running tests...%s\n", "\033[36m", "\033[0m")
	}

	ctestArgs := []string{"--test-dir", buildDir}

	if verbose {
		ctestArgs = append(ctestArgs, "--verbose")
	}

	if filter != "" {
		ctestArgs = append(ctestArgs, "--output-on-failure", "-R", filter)
	} else {
		ctestArgs = append(ctestArgs, "--output-on-failure")
	}

	ctestCmd := exec.Command("ctest", ctestArgs...)
	ctestCmd.Stdout = os.Stdout
	ctestCmd.Stderr = os.Stderr

	if err := ctestCmd.Run(); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	fmt.Printf("%s All tests passed!%s\n", "\033[32m", "\033[0m")
	return nil
}
