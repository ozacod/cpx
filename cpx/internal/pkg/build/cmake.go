package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/ozacod/cpx/internal/pkg/vcpkg"
)

// GetProjectNameFromCMakeLists extracts project name from CMakeLists.txt in current directory
func GetProjectNameFromCMakeLists() string {
	cmakeListsPath := "CMakeLists.txt"
	data, err := os.ReadFile(cmakeListsPath)
	if err != nil {
		return ""
	}

	// Look for: project(PROJECT_NAME ...)
	re := regexp.MustCompile(`project\s*\(\s*([^\s\)]+)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// DetermineBuildType determines the CMake build type and CXX flags based on release flag and optimization level.
// Returns (buildType, cxxFlags)
func DetermineBuildType(release bool, optLevel string) (string, string) {
	buildType := "Debug"
	cxxFlags := ""

	if release {
		buildType = "Release"
	}

	// Handle optimization level
	switch optLevel {
	case "0":
		cxxFlags = "-O0"
		buildType = "Debug"
	case "1":
		cxxFlags = "-O1"
		buildType = "RelWithDebInfo"
	case "2":
		cxxFlags = "-O2"
		buildType = "Release"
	case "3":
		cxxFlags = "-O3"
		buildType = "Release"
	case "s":
		cxxFlags = "-Os"
		buildType = "MinSizeRel"
	case "fast":
		cxxFlags = "-Ofast"
		buildType = "Release"
	}

	return buildType, cxxFlags
}

// GetSanitizerFlags returns the CXX flags and linker flags for the given sanitizer
func GetSanitizerFlags(sanitizer string) (string, string) {
	cxxFlags := ""
	linkerFlags := ""
	switch sanitizer {
	case "asan":
		cxxFlags = " -fsanitize=address -fno-omit-frame-pointer"
		linkerFlags = "-fsanitize=address"
	case "tsan":
		cxxFlags = " -fsanitize=thread"
		linkerFlags = "-fsanitize=thread"
	case "msan":
		cxxFlags = " -fsanitize=memory -fno-omit-frame-pointer"
		linkerFlags = "-fsanitize=memory"
	case "ubsan":
		cxxFlags = " -fsanitize=undefined"
		linkerFlags = "-fsanitize=undefined"
	}
	return cxxFlags, linkerFlags
}

// BuildProject builds the project using CMake
func BuildProject(release bool, jobs int, target string, clean bool, optLevel string, verbose bool, sanitizer string, vcpkgClient *vcpkg.Client) error {
	// Set VCPKG_ROOT from cpx config if not already set
	if err := vcpkgClient.SetupEnv(); err != nil {
		return err
	}

	// Get project name from CMakeLists.txt (optional, for display only)
	projectName := GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	// Determine build output directory based on optimization/release/sanitizer
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

	// Use hidden cache directory for build artifacts
	// .cache/native/<variant>
	cacheBuildDir := filepath.Join(".cache", "native", outDirName)
	// Final executables go to .bin/native/<variant>
	finalBuildDir := filepath.Join(".bin", "native", outDirName)

	if clean {
		if verbose {
			fmt.Printf("%s  Cleaning build directory...%s\n", colorCyan, colorReset)
		}
		os.RemoveAll(cacheBuildDir)
		os.RemoveAll(finalBuildDir)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache build dir: %w", err)
	}

	// Determine build type and optimization
	buildType, cxxFlags := DetermineBuildType(release, optLevel)

	// Add sanitizer flags
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
			// Pass -B explicitly to override preset binaryDir if needed, or ensure it goes to our cache
			// Also pass VCPKG_INSTALLED_DIR to force shared vcpkg location
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
			cmd.Env = os.Environ()
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
	var buildArgs []string
	if verbose {
		buildArgs = []string{"--build", cacheBuildDir, "--config", buildType, "--verbose"}
	} else {
		buildArgs = []string{"--build", cacheBuildDir, "--config", buildType}
	}

	// Add -j flag
	if jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", jobs))
	} else {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", runtime.NumCPU()))
	}

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

	fmt.Printf("%s  ✔ Build complete%s %s[%s]%s\n", colorGreen, colorReset, colorGray, time.Since(buildStart).Round(10*time.Millisecond), colorReset)
	fmt.Printf("  Artifacts in: %s/\n\n", finalBuildDir)
	return nil
}
