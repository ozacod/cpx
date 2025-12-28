// Package cmake provides CMake-only build system integration (no package manager).
package cmake

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build/common"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/templates"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
)

var execCommand = exec.Command

// Builder implements the build.BuildSystem interface for CMake-only projects.
type Builder struct{}

// New creates a new CMake Builder.
func New() *Builder {
	return &Builder{}
}

// Name returns the name of the build system.
func (b *Builder) Name() string {
	return "cmake"
}

// Build compiles the project with the given options.
func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) error {
	// Get project name
	projectName := common.GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	// Determine output directory based on options
	outputDir := build.GetOutputDir(opts.Release, opts.OptLevel, opts.Sanitizer)
	buildDir := filepath.Join(".cache", "native", outputDir)
	finalDir := filepath.Join(".bin", "native", outputDir)

	// Determine build type and flags
	buildType, cxxFlags := common.DetermineBuildType(opts.Release, opts.OptLevel)

	// Add sanitizer flags
	sanCFlags, sanLFlags := common.GetSanitizerFlags(opts.Sanitizer)
	cxxFlags += sanCFlags

	optLabel := common.GetBuildOptLabel(opts.Release, opts.OptLevel, opts.Sanitizer)

	// Clean if requested
	if opts.Clean {
		if err := b.Clean(ctx, build.CleanOptions{All: false}); err != nil {
			return err
		}
	}

	// Create build directory
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	fmt.Printf("\n%s▸ Build%s %s %s(%s)%s %s[opt: %s]%s\n",
		colors.Cyan, colors.Reset, projectName, colors.Gray, buildType, colors.Reset,
		colors.Gray, optLabel, colors.Reset)

	// Check if configure is needed
	needsConfigure := false
	if _, err := os.Stat(filepath.Join(buildDir, "CMakeCache.txt")); os.IsNotExist(err) {
		needsConfigure = true
	}

	totalSteps := 1
	currentStep := 0
	if needsConfigure {
		totalSteps = 2
	}

	// Configure if needed
	if needsConfigure {
		currentStep++
		if opts.Verbose {
			fmt.Printf("%s  • Configuring CMake%s\n", colors.Cyan, colors.Reset)
		} else {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configuring...", colors.Cyan, currentStep, totalSteps, colors.Reset)
		}

		// Determine generator
		generator := "Unix Makefiles"
		if _, err := exec.LookPath("ninja"); err == nil {
			generator = "Ninja"
		}

		configArgs := []string{
			"-B", buildDir,
			"-G", generator,
			"-DCMAKE_BUILD_TYPE=" + buildType,
			"-DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
		}

		if cxxFlags != "" {
			configArgs = append(configArgs, "-DCMAKE_CXX_FLAGS="+cxxFlags, "-DCMAKE_C_FLAGS="+cxxFlags)
		}
		if sanLFlags != "" {
			configArgs = append(configArgs, "-DCMAKE_EXE_LINKER_FLAGS="+sanLFlags, "-DCMAKE_SHARED_LINKER_FLAGS="+sanLFlags)
		}

		configCmd := execCommand("cmake", configArgs...)
		if err := common.RunCMakeConfigure(configCmd, opts.Verbose); err != nil {
			fmt.Println()
			return fmt.Errorf("cmake configure failed: %w", err)
		}

		if !opts.Verbose {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configured ✓\n", colors.Cyan, currentStep, totalSteps, colors.Reset)
		}
	}

	// Build
	buildArgs := []string{"--build", buildDir, "--config", buildType}

	if opts.Jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", opts.Jobs))
	} else {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", runtime.NumCPU()))
	}

	if opts.Target != "" {
		buildArgs = append(buildArgs, "--target", opts.Target)
	}

	if opts.Verbose {
		buildArgs = append(buildArgs, "--verbose")
	}

	currentStep++
	if err := common.RunCMakeBuild(buildArgs, opts.Verbose, currentStep, totalSteps); err != nil {
		return fmt.Errorf("cmake build failed: %w", err)
	}

	// Copy artifacts to output directory
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	executables, err := common.FindExecutables(buildDir)
	if err == nil {
		for _, exe := range executables {
			dest := filepath.Join(finalDir, filepath.Base(exe))
			_ = common.CopyAndSign(exe, dest)
		}
	}

	fmt.Printf("%s  ✔ Build complete%s\n", colors.Green, colors.Reset)
	fmt.Printf("  Artifacts in: %s/\n\n", finalDir)

	return nil
}

// Test runs the project's tests with the given options.
func (b *Builder) Test(ctx context.Context, opts build.TestOptions) error {
	projectName := common.GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	fmt.Printf("%s Running tests for '%s'...%s\n", colors.Cyan, projectName, colors.Reset)

	buildDir := filepath.Join(".cache", "native", "test")

	// Check if configure is needed
	needsConfigure := false
	if _, err := os.Stat(filepath.Join(buildDir, "CMakeCache.txt")); os.IsNotExist(err) {
		needsConfigure = true
	}

	totalSteps := 2
	if needsConfigure {
		totalSteps = 3
	}
	currentStep := 0

	// Configure if needed
	if needsConfigure {
		currentStep++
		if opts.Verbose {
			fmt.Printf("%s  Configuring CMake (with testing enabled)...%s\n", colors.Cyan, colors.Reset)
		} else {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configuring...", colors.Cyan, currentStep, totalSteps, colors.Reset)
		}

		generator := "Unix Makefiles"
		if _, err := exec.LookPath("ninja"); err == nil {
			generator = "Ninja"
		}

		configArgs := []string{
			"-B", buildDir,
			"-G", generator,
			"-DCMAKE_BUILD_TYPE=Debug",
			"-DENABLE_TESTING=ON",
		}

		configCmd := execCommand("cmake", configArgs...)
		if err := common.RunCMakeConfigure(configCmd, opts.Verbose); err != nil {
			fmt.Println()
			return fmt.Errorf("cmake configure failed: %w", err)
		}

		if !opts.Verbose {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configured ✓\n", colors.Cyan, currentStep, totalSteps, colors.Reset)
		}
	}

	// Build tests
	currentStep++
	buildArgs := []string{"--build", buildDir}
	if err := common.RunCMakeBuild(buildArgs, opts.Verbose, currentStep, totalSteps); err != nil {
		return fmt.Errorf("failed to build tests: %w", err)
	}

	// Run tests with CTest
	currentStep++
	if !opts.Verbose {
		fmt.Printf("%s[%d/%d]%s Running tests...\n", colors.Cyan, currentStep, totalSteps, colors.Reset)
	}

	ctestArgs := []string{"--test-dir", buildDir, "--output-on-failure"}
	if opts.Verbose {
		ctestArgs = append(ctestArgs, "-V")
	}
	if opts.Filter != "" {
		ctestArgs = append(ctestArgs, "-R", opts.Filter)
	}

	testCmd := execCommand("ctest", ctestArgs...)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("ctest failed: %w", err)
	}

	fmt.Printf("%s All tests passed!%s\n", colors.Green, colors.Reset)
	return nil
}

// Run builds and runs the project's main executable.
func (b *Builder) Run(ctx context.Context, opts build.RunOptions) error {
	// Build first
	if err := b.Build(ctx, build.BuildOptions{
		Release:   opts.Release,
		OptLevel:  opts.OptLevel,
		Sanitizer: opts.Sanitizer,
		Target:    opts.Target,
		Verbose:   opts.Verbose,
	}); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Find executable
	outputDir := build.GetOutputDir(opts.Release, opts.OptLevel, opts.Sanitizer)
	binDir := filepath.Join(".bin", "native", outputDir)

	var exePath string
	if opts.Target != "" {
		exePath = filepath.Join(binDir, opts.Target)
		if runtime.GOOS == "windows" && !strings.HasSuffix(exePath, ".exe") {
			exePath += ".exe"
		}
	} else {
		// Find first executable
		executables, err := common.FindExecutables(binDir)
		if err != nil {
			return fmt.Errorf("failed to read output directory: %w", err)
		}

		if len(executables) == 0 {
			return fmt.Errorf("no executable found in %s\n  hint: use --target to specify the executable", binDir)
		}

		exePath = executables[0]
		if len(executables) > 1 {
			fmt.Printf("%s Multiple executables found:%s\n", colors.Gray, colors.Reset)
			for i, executable := range executables {
				fmt.Printf("  [%d] %s\n", i+1, filepath.Base(executable))
			}
			fmt.Printf("\nUse --target <name> to specify which one to run\n")
			fmt.Printf("%s Running first: %s%s\n", colors.Yellow, filepath.Base(exePath), colors.Reset)
		}
	}

	fmt.Printf("%s  ▶ Run%s %s%s%s\n\n", colors.Cyan, colors.Reset, colors.Green, filepath.Base(exePath), colors.Reset)
	fmt.Println(strings.Repeat("─", 40))

	runCmd := execCommand(exePath, opts.Args...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	return runCmd.Run()
}

// Bench runs the project's benchmarks.
func (b *Builder) Bench(ctx context.Context, opts build.BenchOptions) error {
	projectName := common.GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	fmt.Printf("%s Running benchmarks for '%s'...%s\n", colors.Cyan, projectName, colors.Reset)

	// Use .cache/native/bench for building benchmarks (separate from normal builds)
	buildDir := filepath.Join(".cache", "native", "bench")
	benchTarget := projectName + "_bench"
	if opts.Target != "" {
		benchTarget = opts.Target
	}

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
		if opts.Verbose {
			fmt.Printf("%s  Configuring CMake (with benchmarks enabled)...%s\n", colors.Cyan, colors.Reset)
		} else {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configuring...", colors.Cyan, currentStep, totalSteps, colors.Reset)
		}

		// Determine generator
		generator := "Unix Makefiles"
		if _, err := exec.LookPath("ninja"); err == nil {
			generator = "Ninja"
		}

		configArgs := []string{
			"-B", buildDir,
			"-G", generator,
			"-DCMAKE_BUILD_TYPE=Release",
			"-DENABLE_BENCHMARKS=ON",
			"-DCMAKE_CXX_FLAGS=-O3",
			"-DCMAKE_C_FLAGS=-O3",
		}

		configCmd := execCommand("cmake", configArgs...)
		if err := common.RunCMakeConfigure(configCmd, opts.Verbose); err != nil {
			fmt.Println()
			return fmt.Errorf("cmake configure failed: %w", err)
		}

		if !opts.Verbose {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configured ✓\n", colors.Cyan, currentStep, totalSteps, colors.Reset)
		}
	}

	// Build benchmarks
	currentStep++
	buildArgs := []string{"--build", buildDir, "--target", benchTarget}
	if opts.Verbose {
		buildArgs = append(buildArgs, "--verbose")
	}
	buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", runtime.NumCPU()))

	if err := common.RunCMakeBuild(buildArgs, opts.Verbose, currentStep, totalSteps); err != nil {
		return fmt.Errorf("failed to build benchmarks: %w", err)
	}

	// Run benchmarks
	currentStep++
	if !opts.Verbose {
		fmt.Printf("%s[%d/%d]%s Running benchmarks...\n", colors.Cyan, currentStep, totalSteps, colors.Reset)
	} else {
		fmt.Printf("%s Running benchmarks...%s\n", colors.Cyan, colors.Reset)
	}

	// Find the benchmark executable
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

	benchCmd := execCommand(benchPath)
	benchCmd.Stdout = os.Stdout
	benchCmd.Stderr = os.Stderr

	fmt.Println() // Add blank line before benchmark output
	if err := benchCmd.Run(); err != nil {
		return fmt.Errorf("benchmark failed: %w", err)
	}

	fmt.Printf("\n%s✓ Benchmarks complete%s\n", colors.Green, colors.Reset)
	return nil
}

// Clean removes build artifacts.
func (b *Builder) Clean(ctx context.Context, opts build.CleanOptions) error {
	fmt.Printf("%sCleaning CMake project...%s\n", colors.Cyan, colors.Reset)

	// Remove cache directory
	common.RemoveDir(".cache/native")

	// Remove build output directory
	common.RemoveDir(".bin/native")

	if opts.All {
		// Remove any build directories
		common.RemoveDir("build")
		common.RemoveDirsMatchingPattern("build-*", true)
	}

	fmt.Printf("%s✓ CMake project cleaned%s\n", colors.Green, colors.Reset)
	return nil
}

// AddDependency tells the user to use a package manager for dependency management.
func (b *Builder) AddDependency(ctx context.Context, name string, version string) error {
	fmt.Printf("%s⚠ CMake-only projects don't have built-in dependency management%s\n\n", colors.Yellow, colors.Reset)
	fmt.Printf("To add dependencies, consider one of these options:\n\n")

	fmt.Printf("%s1. Use a package manager project:%s\n", colors.Cyan, colors.Reset)
	fmt.Printf("   Create a new project with vcpkg, Meson, or Bazel:\n")
	fmt.Printf("     cpx new\n\n")

	fmt.Printf("%s2. Use CMake FetchContent (recommended for CMake-only):%s\n", colors.Cyan, colors.Reset)
	fmt.Printf("   Add this to your CMakeLists.txt:\n\n")
	fmt.Printf("   include(FetchContent)\n")
	fmt.Printf("   FetchContent_Declare(\n")
	fmt.Printf("       %s\n", name)
	fmt.Printf("       GIT_REPOSITORY https://github.com/.../%s.git\n", name)
	fmt.Printf("       GIT_TAG <version>\n")
	fmt.Printf("   )\n")
	fmt.Printf("   FetchContent_MakeAvailable(%s)\n\n", name)

	fmt.Printf("%s3. Use git submodules:%s\n", colors.Cyan, colors.Reset)
	fmt.Printf("   git submodule add https://github.com/.../%s.git external/%s\n\n", name, name)

	fmt.Printf("%s4. Install system-wide:%s\n", colors.Cyan, colors.Reset)
	fmt.Printf("   # macOS\n")
	fmt.Printf("   brew install %s\n", name)
	fmt.Printf("   # Ubuntu/Debian\n")
	fmt.Printf("   apt install lib%s-dev\n\n", name)

	return nil
}

// RemoveDependency is not supported for CMake-only projects.
func (b *Builder) RemoveDependency(ctx context.Context, name string) error {
	return fmt.Errorf("CMake-only projects don't have built-in dependency management\n  hint: manually remove the dependency from CMakeLists.txt or FetchContent")
}

// ListDependencies is not fully supported for CMake-only projects.
func (b *Builder) ListDependencies(ctx context.Context) ([]build.Dependency, error) {
	fmt.Printf("%sNote: CMake-only projects don't have a standard dependency manifest%s\n", colors.Yellow, colors.Reset)
	fmt.Printf("Check your CMakeLists.txt for:\n")
	fmt.Printf("  - find_package() calls\n")
	fmt.Printf("  - FetchContent_Declare() calls\n")
	fmt.Printf("  - add_subdirectory() for external projects\n")
	return nil, nil
}

// SearchDependencies is not supported for CMake-only projects.
func (b *Builder) SearchDependencies(ctx context.Context, query string) ([]build.Dependency, error) {
	return nil, fmt.Errorf("CMake-only projects don't have a package registry\n  hint: search on GitHub, vcpkg ports, or Conan packages")
}

// DependencyInfo is not supported for CMake-only projects.
func (b *Builder) DependencyInfo(ctx context.Context, name string) (*build.DependencyInfo, error) {
	return nil, fmt.Errorf("CMake-only projects don't have dependency info\n  hint: check the package's GitHub page or documentation")
}

// ListTargets returns the list of build targets from CMake.
func (b *Builder) ListTargets(ctx context.Context) ([]string, error) {
	// Try to find a build directory
	buildDirs := []string{".cache/native/debug", ".cache/native/release", "build"}
	var buildDir string

	for _, dir := range buildDirs {
		if _, err := os.Stat(dir); err == nil {
			buildDir = dir
			break
		}
	}

	if buildDir == "" {
		return nil, fmt.Errorf("no build directory found. Run 'cpx build' first")
	}

	// Use cmake --build with --target help (works for most generators)
	cmd := execCommand("cmake", "--build", buildDir, "--target", "help")
	output, err := cmd.Output()
	if err != nil {
		// Fallback: try to parse CMakeLists.txt for targets
		return b.parseTargetsFromCMakeLists()
	}

	// Parse the output for target names
	var targets []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "...") {
			// Extract target name
			if idx := strings.Index(line, ":"); idx != -1 {
				targets = append(targets, strings.TrimSpace(line[:idx]))
			}
		}
	}

	return targets, nil
}

// parseTargetsFromCMakeLists attempts to extract targets from CMakeLists.txt
func (b *Builder) parseTargetsFromCMakeLists() ([]string, error) {
	data, err := os.ReadFile("CMakeLists.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to read CMakeLists.txt: %w", err)
	}

	var targets []string
	content := string(data)

	// Look for add_executable and add_library calls
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "add_executable(") || strings.HasPrefix(line, "add_library(") {
			// Extract target name (first argument)
			start := strings.Index(line, "(")
			if start != -1 {
				rest := line[start+1:]
				end := strings.IndexAny(rest, " \t)")
				if end != -1 {
					target := rest[:end]
					targets = append(targets, target)
				}
			}
		}
	}

	return targets, nil
}

// GenerateGitignore generates the .gitignore file for CMake projects.
func (b *Builder) GenerateGitignore(ctx context.Context, projectPath string) error {
	gitignore := templates.GenerateCMakeGitignore()
	if err := os.WriteFile(filepath.Join(projectPath, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}
	return nil
}

// GenerateBuildSrc generates the build files for source code.
func (b *Builder) GenerateBuildSrc(ctx context.Context, projectPath string, config build.InitConfig) error {
	// Generate CMakeLists.txt
	cmakeLists := templates.GenerateCMakeLists(
		config.Name,
		config.CppStandard,
		!config.IsLibrary,
		config.TestFramework != "" && config.TestFramework != "none",
		config.Benchmark,
		config.Benchmark != "" && config.Benchmark != "none",
		config.Version,
	)
	if err := os.WriteFile(filepath.Join(projectPath, "CMakeLists.txt"), []byte(cmakeLists), 0644); err != nil {
		return fmt.Errorf("failed to write CMakeLists.txt: %w", err)
	}

	return nil
}

// GenerateBuildTest generates the build files for tests.
func (b *Builder) GenerateBuildTest(ctx context.Context, projectPath string, config build.InitConfig) error {
	if config.TestFramework == "" || config.TestFramework == "none" {
		return nil
	}

	if err := os.MkdirAll(filepath.Join(projectPath, "tests"), 0755); err != nil {
		return fmt.Errorf("failed to create tests directory: %w", err)
	}

	// Generate tests/CMakeLists.txt
	testCMake := templates.GenerateTestCMake(config.Name, config.TestFramework)
	if err := os.WriteFile(filepath.Join(projectPath, "tests/CMakeLists.txt"), []byte(testCMake), 0644); err != nil {
		return fmt.Errorf("failed to write tests/CMakeLists.txt: %w", err)
	}

	return nil
}

// GenerateBuildBench generates the build files for benchmarks.
func (b *Builder) GenerateBuildBench(ctx context.Context, projectPath string, config build.InitConfig) error {
	if config.Benchmark == "" || config.Benchmark == "none" {
		return nil
	}

	if err := os.MkdirAll(filepath.Join(projectPath, "bench"), 0755); err != nil {
		return fmt.Errorf("failed to create bench directory: %w", err)
	}

	// Generate bench/CMakeLists.txt
	benchCMake := templates.GenerateBenchCMake(config.Name, config.Benchmark)
	if err := os.WriteFile(filepath.Join(projectPath, "bench/CMakeLists.txt"), []byte(benchCMake), 0644); err != nil {
		return fmt.Errorf("failed to write bench/CMakeLists.txt: %w", err)
	}

	return nil
}

// Ensure Builder implements BuildSystem interface
var _ build.BuildSystem = (*Builder)(nil)
