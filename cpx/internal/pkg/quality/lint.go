package quality

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VcpkgSetup is an interface for vcpkg operations needed by lint
type VcpkgSetup interface {
	SetupEnv() error
	GetPath() (string, error)
}

// LintCode runs clang-tidy static analysis
func LintCode(fix bool, vcpkg VcpkgSetup) error {
	// Check if clang-tidy is available
	if _, err := exec.LookPath("clang-tidy"); err != nil {
		return fmt.Errorf("clang-tidy not found. Please install it first")
	}

	fmt.Printf("%s Running static analysis...%s\n", Cyan, Reset)

	// Set up vcpkg environment
	if err := vcpkg.SetupEnv(); err != nil {
		return fmt.Errorf("failed to setup vcpkg: %w", err)
	}

	// Check for compile_commands.json and regenerate if needed
	// Use .cache/native/debug for consistency with build command
	buildDir := filepath.Join(".cache", "native", "debug")
	compileDb := filepath.Join(buildDir, "compile_commands.json")
	needsRegenerate := false

	if _, err := os.Stat(compileDb); os.IsNotExist(err) {
		needsRegenerate = true
		fmt.Printf("%s  Generating compile_commands.json...%s\n", Cyan, Reset)
	} else {
		// Check if CMakeCache.txt exists - if not, we need to configure
		if _, err := os.Stat(filepath.Join(buildDir, "CMakeCache.txt")); os.IsNotExist(err) {
			needsRegenerate = true
			fmt.Printf("%s  Regenerating compile_commands.json (CMake not configured)...%s\n", Cyan, Reset)
		}
	}

	if needsRegenerate {
		// Get vcpkg root for toolchain file
		vcpkgPath, err := vcpkg.GetPath()
		if err != nil {
			return fmt.Errorf("vcpkg not configured: %w", err)
		}
		vcpkgRoot := filepath.Dir(vcpkgPath)
		toolchainFile := filepath.Join(vcpkgRoot, "scripts", "buildsystems", "vcpkg.cmake")

		// Check if toolchain file exists
		if _, err := os.Stat(toolchainFile); os.IsNotExist(err) {
			return fmt.Errorf("vcpkg toolchain file not found: %s\n  Make sure vcpkg is properly installed", toolchainFile)
		}

		// Configure CMake with vcpkg toolchain
		// Use shared vcpkg_installed directory
		cwd, _ := os.Getwd()
		vcpkgInstalledDir := filepath.Join(cwd, ".cache", "native", "vcpkg_installed")
		vcpkgInstallArg := "-DVCPKG_INSTALLED_DIR=" + vcpkgInstalledDir

		cmakeArgs := []string{
			"-B", buildDir,
			"-DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
			"-DCMAKE_TOOLCHAIN_FILE=" + toolchainFile,
			vcpkgInstallArg,
		}

		// Check if CMakePresets.json exists and use it
		if _, err := os.Stat("CMakePresets.json"); err == nil {
			// Use preset if available
			cmakeArgs = []string{
				"--preset", "default",
				"-B", buildDir,
				"-DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
				vcpkgInstallArg,
			}
		}

		cmd := exec.Command("cmake", cmakeArgs...)
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to generate compile_commands.json: %w\n  Try running 'cpx build' first to configure the project", err)
		}
	}

	// Find source files (only git-tracked files, respect .gitignore)
	var files []string
	trackedFiles, err := GetGitTrackedCppFiles()
	if err != nil {
		// If not in git repo, fall back to scanning src/include directories
		fmt.Printf("%s Warning: Not in a git repository. Scanning src/, include/, and current directory.%s\n", Yellow, Reset)
		for _, dir := range []string{".", "src", "include"} {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue
			}
			_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				// Skip build directories and other common ignored paths
				if strings.Contains(path, "/build/") || strings.Contains(path, "\\build\\") || strings.Contains(path, "/.cache/") {
					return nil
				}
				ext := filepath.Ext(path)
				if ext == ".cpp" || ext == ".cc" || ext == ".cxx" || ext == ".c++" {
					files = append(files, path)
				}
				return nil
			})
		}
	} else {
		// Filter out files in build directories and other common ignored paths
		for _, file := range trackedFiles {
			// Skip files in build/, out/, bin/, .vcpkg/, etc.
			if strings.HasPrefix(file, "build/") ||
				strings.HasPrefix(file, "out/") ||
				strings.HasPrefix(file, "bin/") ||
				strings.HasPrefix(file, ".vcpkg/") ||
				strings.HasPrefix(file, ".cache/") ||
				strings.Contains(file, "/build/") ||
				strings.Contains(file, "\\build\\") {
				continue
			}
			files = append(files, file)
		}
	}

	if len(files) == 0 {
		fmt.Printf("%s No source files found%s\n", Green, Reset)
		return nil
	}

	// Verify compile_commands.json exists
	if _, err := os.Stat(compileDb); os.IsNotExist(err) {
		return fmt.Errorf("compile_commands.json not found at %s\n  Run 'cpx build' first to generate it", compileDb)
	}

	// Get system include paths from the compiler to help clang-tidy find standard headers
	// This is needed because compile_commands.json might not have all system includes
	systemIncludes := GetSystemIncludePaths()

	// Run clang-tidy with absolute path to build directory
	absBuildDir, _ := filepath.Abs(buildDir)
	tidyArgs := []string{"-p", absBuildDir}
	if fix {
		tidyArgs = append(tidyArgs, "-fix")
	}
	// Add system include paths as extra arguments
	for _, include := range systemIncludes {
		tidyArgs = append(tidyArgs, "--extra-arg=-isystem"+include)
	}
	tidyArgs = append(tidyArgs, files...)

	cmd := exec.Command("clang-tidy", tidyArgs...)
	output, err := cmd.CombinedOutput()

	// Write output to stderr (warnings/errors) and stdout (info)
	os.Stderr.Write(output)

	// Check if output contains warnings or errors
	outputStr := string(output)
	hasWarnings := strings.Contains(outputStr, "warning:") ||
		strings.Contains(outputStr, "error:") ||
		strings.Contains(outputStr, "note:")

	if err != nil {
		// clang-tidy returns non-zero on errors or when warnings are treated as errors
		if hasWarnings {
			fmt.Printf("%s  Analysis complete with issues found%s\n", Yellow, Reset)
		} else {
			fmt.Printf("%s  Analysis failed%s\n", Yellow, Reset)
		}
		return nil
	}

	if hasWarnings {
		fmt.Printf("%s  Analysis complete with warnings%s\n", Yellow, Reset)
		return nil
	}

	fmt.Printf("%s No issues found!%s\n", Green, Reset)
	return nil
}

// GetSystemIncludePaths gets system include paths from the compiler
func GetSystemIncludePaths() []string {
	var includes []string

	// Try to get system includes from clang++
	// Use -E -x c++ - -v to get verbose include search paths
	cmd := exec.Command("clang++", "-E", "-x", "c++", "-", "-v")
	nullFile, err := os.Open(os.DevNull)
	if err != nil {
		return includes
	}
	defer nullFile.Close()
	cmd.Stdin = nullFile
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If clang++ fails, try clang
		cmd = exec.Command("clang", "-E", "-x", "c++", "-", "-v")
		nullFile2, err2 := os.Open(os.DevNull)
		if err2 != nil {
			return includes
		}
		defer nullFile2.Close()
		cmd.Stdin = nullFile2
		output, err = cmd.CombinedOutput()
		if err != nil {
			return includes
		}
	}

	// Parse the output to find include paths
	// The output format is:
	// #include <...> search starts here:
	//  /path/to/include
	// End of search list.
	lines := strings.Split(string(output), "\n")
	inIncludeSection := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "#include <...> search starts here:") {
			inIncludeSection = true
			continue
		}
		if strings.Contains(line, "End of search list.") {
			break
		}
		if inIncludeSection && line != "" && !strings.HasPrefix(line, "#") {
			// Remove leading/trailing whitespace and check if it's a valid path
			includePath := strings.TrimSpace(line)
			if filepath.IsAbs(includePath) || strings.HasPrefix(includePath, "/") {
				includes = append(includes, includePath)
			}
		}
	}

	return includes
}
