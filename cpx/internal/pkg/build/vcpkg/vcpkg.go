// Package vcpkg provides vcpkg build system integration.
package vcpkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ozacod/cpx/internal/pkg/build/common"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/templates"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
	"github.com/ozacod/cpx/pkg/config"
)

var execCommand = exec.Command

// Builder implements the build.BuildSystem interface for vcpkg.
type Builder struct {
	globalConfig *config.GlobalConfig
}

func New() *Builder {
	return &Builder{}
}

// ensureConfig ensures the global config is loaded
func (b *Builder) ensureConfig() error {
	if b.globalConfig != nil {
		return nil
	}
	globalConfig, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("failed to load global config: %w", err)
	}
	b.globalConfig = globalConfig
	return nil
}

func (b *Builder) SetupEnv() error {
	if err := b.ensureConfig(); err != nil {
		return err
	}

	// Set VCPKG_ROOT if not already set and we have it in config
	if os.Getenv("VCPKG_ROOT") == "" {
		if b.globalConfig.VcpkgRoot == "" {
			return fmt.Errorf("vcpkg_root not set in config. Run: cpx config set-vcpkg-root <path>")
		}
		if err := os.Setenv("VCPKG_ROOT", b.globalConfig.VcpkgRoot); err != nil {
			return fmt.Errorf("failed to set VCPKG_ROOT: %w", err)
		}
	}

	// Set VCPKG_FEATURE_FLAGS=manifests if not already set
	if os.Getenv("VCPKG_FEATURE_FLAGS") == "" {
		if err := os.Setenv("VCPKG_FEATURE_FLAGS", "manifests"); err != nil {
			return fmt.Errorf("failed to set VCPKG_FEATURE_FLAGS: %w", err)
		}
	}

	// Set VCPKG_DISABLE_REGISTRY_UPDATE=1 if not already set
	if os.Getenv("VCPKG_DISABLE_REGISTRY_UPDATE") == "" {
		if err := os.Setenv("VCPKG_DISABLE_REGISTRY_UPDATE", "1"); err != nil {
			return fmt.Errorf("failed to set VCPKG_DISABLE_REGISTRY_UPDATE: %w", err)
		}
	}

	if os.Getenv("CPX_DEBUG") != "" {
		fmt.Printf("%s[DEBUG] VCPKG Environment:%s\n", colors.Cyan, colors.Reset)
		fmt.Printf("  VCPKG_ROOT=%s\n", os.Getenv("VCPKG_ROOT"))
		fmt.Printf("  VCPKG_FEATURE_FLAGS=%s\n", os.Getenv("VCPKG_FEATURE_FLAGS"))
		fmt.Printf("  VCPKG_DISABLE_REGISTRY_UPDATE=%s\n", os.Getenv("VCPKG_DISABLE_REGISTRY_UPDATE"))
	}

	return nil
}

type configureOptions struct {
	buildDir         string
	buildType        string
	cxxFlags         string
	linkerFlags      string
	enableTesting    bool
	enableBenchmarks bool
	verbose          bool
}

func (b *Builder) configureCMake(opts configureOptions) error {
	// Determine absolute path for shared vcpkg_installed directory
	cwd, _ := os.Getwd()
	vcpkgInstalledDir := filepath.Join(cwd, ".cache", "native", "vcpkg_installed")
	vcpkgInstallArg := "-DVCPKG_INSTALLED_DIR=" + vcpkgInstalledDir

	var cmdArgs []string

	hasPresets := false
	if _, err := os.Stat("CMakePresets.json"); err == nil {
		hasPresets = true
		cmdArgs = []string{"--preset=default", "-B", opts.buildDir, vcpkgInstallArg}
	} else {
		cmdArgs = []string{"-B", opts.buildDir, "-DCMAKE_BUILD_TYPE=" + opts.buildType, vcpkgInstallArg}
	}

	if opts.cxxFlags != "" {
		cmdArgs = append(cmdArgs, "-DCMAKE_CXX_FLAGS="+opts.cxxFlags, "-DCMAKE_C_FLAGS="+opts.cxxFlags)
	}
	if opts.linkerFlags != "" {
		cmdArgs = append(cmdArgs, "-DCMAKE_EXE_LINKER_FLAGS="+opts.linkerFlags, "-DCMAKE_SHARED_LINKER_FLAGS="+opts.linkerFlags)
	}

	if opts.enableTesting {
		cmdArgs = append(cmdArgs, "-DENABLE_TESTING=ON")
	}
	if opts.enableBenchmarks {
		cmdArgs = append(cmdArgs, "-DENABLE_BENCHMARKS=ON")
		// Force Release build type for benchmarks
		if !hasPresets {
			// Update build type to Release for benchmarks
			for i, arg := range cmdArgs {
				if strings.HasPrefix(arg, "-DCMAKE_BUILD_TYPE=") {
					cmdArgs[i] = "-DCMAKE_BUILD_TYPE=Release"
					break
				}
			}
		} else {
			cmdArgs = append(cmdArgs, "-DCMAKE_BUILD_TYPE=Release")
		}
	}

	cmd := execCommand("cmake", cmdArgs...)
	cmd.Env = os.Environ()

	presetInfo := ""
	if hasPresets {
		presetInfo = " (preset 'default')"
	}

	if err := common.RunCMakeConfigure(cmd, opts.verbose); err != nil {
		return fmt.Errorf("cmake configure failed%s: %w", presetInfo, err)
	}

	return nil
}

func (b *Builder) copyBuildArtifacts(cacheBuildDir, finalBuildDir string) error {
	if err := os.MkdirAll(finalBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create final build dir: %w", err)
	}

	executables, err := common.FindExecutables(cacheBuildDir)
	if err == nil {
		for _, exe := range executables {
			dest := filepath.Join(finalBuildDir, filepath.Base(exe))
			_ = common.CopyAndSign(exe, dest)
		}
	}

	libraries, err := common.FindLibraries(cacheBuildDir)
	if err == nil {
		for _, lib := range libraries {
			dest := filepath.Join(finalBuildDir, filepath.Base(lib))
			_ = common.CopyAndSign(lib, dest)
		}
	}

	return nil
}

func (b *Builder) GetPath() (string, error) {
	if err := b.ensureConfig(); err != nil {
		return "", err
	}

	vcpkgRoot := b.globalConfig.VcpkgRoot

	// If not set in config, check environment variable as fallback
	if vcpkgRoot == "" {
		if envRoot := os.Getenv("VCPKG_ROOT"); envRoot != "" {
			vcpkgRoot = envRoot
		}
	}

	if vcpkgRoot == "" {
		return "", fmt.Errorf("vcpkg_root not set in config. Run: cpx config set-vcpkg-root <path>")
	}

	// Convert to absolute path
	absVcpkgRoot, err := filepath.Abs(vcpkgRoot)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute vcpkg root path: %w", err)
	}

	vcpkgPath := filepath.Join(absVcpkgRoot, "vcpkg")
	if runtime.GOOS == "windows" {
		vcpkgPath += ".exe"
	}

	if _, err := os.Stat(vcpkgPath); os.IsNotExist(err) {
		return "", fmt.Errorf("vcpkg not found at %s. Make sure vcpkg is installed and bootstrapped", vcpkgPath)
	}

	return vcpkgPath, nil
}

func (b *Builder) RunCommand(args []string) error {
	vcpkgPath, err := b.GetPath()
	if err != nil {
		return err
	}

	cmd := execCommand(vcpkgPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	// Remove VCPKG_ROOT from environment to use the one from config
	cmd.Env = os.Environ()
	for i, env := range cmd.Env {
		if strings.HasPrefix(env, "VCPKG_ROOT=") {
			cmd.Env = append(cmd.Env[:i], cmd.Env[i+1:]...)
			break
		}
	}
	return cmd.Run()
}

func (b *Builder) GenerateGitignore(ctx context.Context, projectPath string) error {
	gitignore := templates.GenerateGitignore()
	if err := os.WriteFile(filepath.Join(projectPath, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}
	return nil
}

func (b *Builder) GenerateBuildSrc(ctx context.Context, projectPath string, config build.InitConfig) error {
	hasTest := config.TestFramework != "" && config.TestFramework != "none"
	hasBench := config.Benchmark != "" && config.Benchmark != "none"

	// Generate CMakeLists.txt
	cmakeLists := templates.GenerateVcpkgCMakeLists(templates.CMakeOptions{
		ProjectName:        config.Name,
		CppStandard:        config.CppStandard,
		IsExe:              !config.IsLibrary,
		IncludeTests:       hasTest,
		BenchmarkFramework: config.Benchmark,
		IncludeBench:       hasBench,
		ProjectVersion:     config.Version,
	})
	if err := os.WriteFile(filepath.Join(projectPath, "CMakeLists.txt"), []byte(cmakeLists), 0644); err != nil {
		return fmt.Errorf("failed to write CMakeLists.txt: %w", err)
	}

	// Generate CMakePresets.json
	cmakePresets := templates.GenerateCMakePresets()
	if err := os.WriteFile(filepath.Join(projectPath, "CMakePresets.json"), []byte(cmakePresets), 0644); err != nil {
		return fmt.Errorf("failed to write CMakePresets.json: %w", err)
	}

	return nil
}

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

func (b *Builder) Build(ctx context.Context, opts build.BuildOptions) error {
	// Set VCPKG_ROOT from cpx config if not already set
	if err := b.SetupEnv(); err != nil {
		return err
	}

	// Get project name from CMakeLists.txt (optional, for display only)
	projectName := common.GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	// Determine build output directory based on optimization/release/sanitizer
	// For test builds, use "test" directory; for benchmark builds, use "bench"
	var outDirName string
	if opts.EnableTesting {
		outDirName = "test"
	} else if opts.EnableBenchmarks {
		outDirName = "bench"
	} else {
		outDirName = build.GetOutputDir(opts.Release, opts.OptLevel, opts.Sanitizer)
	}

	// Use hidden cache directory for build artifacts
	// .cache/native/<variant>
	cacheBuildDir := filepath.Join(".cache", "native", outDirName)
	// Final executables go to .bin/native/<variant>
	finalBuildDir := filepath.Join(".bin", "native", outDirName)

	if opts.Clean {
		if opts.Verbose {
			fmt.Printf("%s  Cleaning build directory...%s\n", colors.Cyan, colors.Reset)
		}
		_ = os.RemoveAll(cacheBuildDir)
		_ = os.RemoveAll(finalBuildDir)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache build dir: %w", err)
	}

	// Determine build type and optimization
	// For benchmarks, force Release mode
	release := opts.Release
	if opts.EnableBenchmarks {
		release = true
	}
	buildType, cxxFlags := common.DetermineBuildType(release, opts.OptLevel)

	// Add sanitizer flags
	sanCFlags, sanLFlags := common.GetSanitizerFlags(opts.Sanitizer)
	cxxFlags += sanCFlags
	linkerFlags := sanLFlags

	optLabel := "default (-O0)"
	if release {
		optLabel = "-O2 (Release)"
	}
	if opts.OptLevel != "" {
		optLabel = "-O" + opts.OptLevel
	}
	if opts.Sanitizer != "" {
		optLabel += "+" + opts.Sanitizer
	}

	// Customize header based on build type
	buildHeader := "Build"
	if opts.EnableTesting {
		buildHeader = "Build (tests)"
	} else if opts.EnableBenchmarks {
		buildHeader = "Build (bench)"
	}

	fmt.Printf("\n%s▸ %s%s %s %s(%s)%s %s[opt: %s]%s\n",
		colors.Cyan, buildHeader, colors.Reset, projectName, colors.Gray, buildType, colors.Reset,
		colors.Gray, optLabel, colors.Reset)

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
		configMsg := "Configuring CMake"
		if opts.EnableTesting {
			configMsg = "Configuring CMake (with testing enabled)"
		} else if opts.EnableBenchmarks {
			configMsg = "Configuring CMake (with benchmarks enabled)"
		}

		if opts.Verbose {
			fmt.Printf("%s  • %s%s\n", colors.Cyan, configMsg, colors.Reset)
		} else {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configuring...", colors.Cyan, currentStep, totalSteps, colors.Reset)
		}

		if err := b.configureCMake(configureOptions{
			buildDir:         cacheBuildDir,
			buildType:        buildType,
			cxxFlags:         cxxFlags,
			linkerFlags:      linkerFlags,
			enableTesting:    opts.EnableTesting,
			enableBenchmarks: opts.EnableBenchmarks,
			verbose:          opts.Verbose,
		}); err != nil {
			fmt.Println()
			return err
		}

		if !opts.Verbose {
			fmt.Printf("\r\033[2K%s[%d/%d]%s Configured ✓\n", colors.Cyan, currentStep, totalSteps, colors.Reset)
		}
	}

	// Build specific target if provided
	buildStart := time.Now()
	// Build in .cache directory
	var buildArgs []string
	if opts.Verbose {
		buildArgs = []string{"--build", cacheBuildDir, "--config", buildType, "--verbose"}
	} else {
		buildArgs = []string{"--build", cacheBuildDir, "--config", buildType}
	}

	// Add -j flag
	if opts.Jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", opts.Jobs))
	} else {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", runtime.NumCPU()))
	}

	if opts.Target != "" {
		buildArgs = append(buildArgs, "--target", opts.Target)
	}

	currentStep++
	if err := common.RunCMakeBuild(buildArgs, opts.Verbose, currentStep, totalSteps); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Copy artifacts to final build directory (skip for test/bench builds)
	if !opts.EnableTesting && !opts.EnableBenchmarks {
		if err := b.copyBuildArtifacts(cacheBuildDir, finalBuildDir); err != nil {
			return err
		}
		fmt.Printf("%s  ✔ Build complete%s %s[%s]%s\n", colors.Green, colors.Reset, colors.Gray, time.Since(buildStart).Round(10*time.Millisecond), colors.Reset)
		fmt.Printf("  Artifacts in: %s/\n\n", finalBuildDir)
	} else {
		fmt.Printf("%s  ✔ Build complete%s %s[%s]%s\n", colors.Green, colors.Reset, colors.Gray, time.Since(buildStart).Round(10*time.Millisecond), colors.Reset)
	}

	return nil
}

func (b *Builder) Test(ctx context.Context, opts build.TestOptions) error {
	projectName := common.GetProjectNameFromCMakeLists()
	if projectName == "" {
		return fmt.Errorf("failed to get project name from CMakeLists.txt")
	}

	// Use Build with EnableTesting to build tests
	buildOpts := build.BuildOptions{
		EnableTesting: true,
		Target:        projectName + "_tests",
		Verbose:       opts.Verbose,
	}

	if err := b.Build(ctx, buildOpts); err != nil {
		return fmt.Errorf("failed to build tests: %w", err)
	}

	// Run tests with CTest
	buildDir := filepath.Join(".cache", "native", "test")

	fmt.Printf("%s▸ Running tests...%s\n", colors.Cyan, colors.Reset)

	ctestArgs := []string{"--test-dir", buildDir}

	if opts.Verbose {
		ctestArgs = append(ctestArgs, "--verbose")
	}

	if opts.Filter != "" {
		ctestArgs = append(ctestArgs, "--output-on-failure", "-R", opts.Filter)
	} else {
		ctestArgs = append(ctestArgs, "--output-on-failure")
	}

	ctestCmd := execCommand("ctest", ctestArgs...)
	ctestCmd.Stdout = os.Stdout
	ctestCmd.Stderr = os.Stderr

	if err := ctestCmd.Run(); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	fmt.Printf("%s✓ All tests passed!%s\n", colors.Green, colors.Reset)
	return nil
}

func (b *Builder) Run(ctx context.Context, opts build.RunOptions) error {
	// Get project name from CMakeLists.txt (optional, for display only)
	projectName := common.GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	// Use Build to compile the project
	buildOpts := build.BuildOptions{
		Release:   opts.Release,
		OptLevel:  opts.OptLevel,
		Sanitizer: opts.Sanitizer,
		Target:    opts.Target,
		Verbose:   opts.Verbose,
	}

	if err := b.Build(ctx, buildOpts); err != nil {
		return err
	}

	// Determine where artifacts are
	outDirName := build.GetOutputDir(opts.Release, opts.OptLevel, opts.Sanitizer)
	finalBuildDir := filepath.Join(".bin", "native", outDirName)

	// Find executable to run (in finalBuildDir)
	var execPath string

	// If target specified, look for that specific executable
	if opts.Target != "" {
		targetName := opts.Target
		if runtime.GOOS == "windows" && !strings.HasSuffix(targetName, ".exe") {
			targetName += ".exe"
		}
		execPath = filepath.Join(finalBuildDir, targetName)
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			return fmt.Errorf("target executable '%s' not found in %s", opts.Target, finalBuildDir)
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
			executables, err := common.FindExecutables(finalBuildDir)
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
				fmt.Printf("%s Multiple executables found:%s\n", colors.Gray, colors.Reset)
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

	fmt.Printf("%s▸ Run%s %s%s%s\n\n", colors.Cyan, colors.Reset, colors.Green, filepath.Base(execPath), colors.Reset)
	fmt.Println(strings.Repeat("─", 40))

	runCmd := execCommand(execPath, opts.Args...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin
	return runCmd.Run()
}

func (b *Builder) Bench(ctx context.Context, opts build.BenchOptions) error {
	projectName := common.GetProjectNameFromCMakeLists()
	if projectName == "" {
		return fmt.Errorf("failed to get project name from CMakeLists.txt")
	}

	benchTarget := projectName + "_bench"
	if opts.Target != "" {
		benchTarget = opts.Target
	}

	// Use Build with EnableBenchmarks to build benchmarks (forces Release mode)
	buildOpts := build.BuildOptions{
		EnableBenchmarks: true,
		Target:           benchTarget,
		Verbose:          opts.Verbose,
	}

	if err := b.Build(ctx, buildOpts); err != nil {
		return fmt.Errorf("failed to build benchmarks: %w", err)
	}

	// Run benchmarks
	buildDir := filepath.Join(".cache", "native", "bench")

	fmt.Printf("%s▸ Running benchmarks...%s\n", colors.Cyan, colors.Reset)

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
		return fmt.Errorf("benchmarks failed: %w", err)
	}

	fmt.Printf("\n%s✓ Benchmarks completed!%s\n", colors.Green, colors.Reset)
	return nil
}

func (b *Builder) Clean(ctx context.Context, opts build.CleanOptions) error {
	fmt.Printf("%sCleaning CMake/vcpkg project...%s\n", colors.Cyan, colors.Reset)

	// Remove bin directory (artifacts)
	common.RemoveDir(filepath.Join(".bin", "native"))

	// Remove intermediate build directories (keep vcpkg_installed unless --all)
	// We iterate common variants instead of blowing away .cache/native
	variants := []string{"debug", "release", "O0", "O1", "O2", "O3", "Os", "Ofast", "test", "bench"}
	for _, v := range variants {
		common.RemoveDir(filepath.Join(".cache", "native", v))
	}

	if opts.All {
		// Clean everything including vcpkg dependencies and CI artifacts
		dirsToRemove := []string{
			filepath.Join(".cache", "native"),
			filepath.Join(".cache", "ci"),
			filepath.Join(".bin", "ci"),
			"out",
			"cmake-build-debug",
			"cmake-build-release",
		}
		for _, dir := range dirsToRemove {
			common.RemoveDir(dir)
		}

		// Remove build-* directories
		common.RemoveDirsMatchingPattern("build-*", true)
	}

	fmt.Printf("%s✓ CMake project cleaned%s\n", colors.Green, colors.Reset)
	return nil
}

func (b *Builder) AddDependency(ctx context.Context, name string, version string) error {
	// Set up environment
	if err := b.SetupEnv(); err != nil {
		return err
	}

	// Use vcpkg add port command
	vcpkgArgs := []string{"add", "port", name}
	if err := b.RunCommand(vcpkgArgs); err != nil {
		return fmt.Errorf("failed to add dependency: %w", err)
	}

	fmt.Printf("%s✓ Added %s%s\n", colors.Green, name, colors.Reset)

	// Print usage info from vcpkg GitHub
	b.printUsageInfo(name)

	return nil
}

// printUsageInfo fetches and prints usage info from GitHub for vcpkg packages
func (b *Builder) printUsageInfo(pkgName string) {
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/microsoft/vcpkg/master/ports/%s/usage", pkgName))
	if err != nil || resp.StatusCode != 200 {
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	content := strings.TrimSpace(string(data))
	if content != "" {
		fmt.Printf("\n%sUSAGE INFO FOR %s:%s\n", colors.Cyan, pkgName, colors.Reset)
		fmt.Println(content)
		fmt.Println()
	}

	// Print link to cpx website for more info
	fmt.Printf("%s📦 Find sample usage and more info at:%s\n", colors.Cyan, colors.Reset)
	fmt.Printf("   https://cpx-dev.vercel.app/packages#package/%s\n\n", pkgName)
}

func (b *Builder) RemoveDependency(ctx context.Context, name string) error {
	// Check for vcpkg.json (Manifest mode)
	if _, err := os.Stat("vcpkg.json"); err != nil {
		return fmt.Errorf("vcpkg.json not found - manifest mode required")
	}

	// Read manifest
	data, err := os.ReadFile("vcpkg.json")
	if err != nil {
		return fmt.Errorf("failed to read vcpkg.json: %w", err)
	}

	// Parse JSON
	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse vcpkg.json: %w", err)
	}

	// Get dependencies
	deps, ok := manifest["dependencies"]
	if !ok {
		return fmt.Errorf("no dependencies found in vcpkg.json")
	}

	depList, ok := deps.([]interface{})
	if !ok {
		return fmt.Errorf("invalid dependencies format in vcpkg.json")
	}

	// Filter out the dependency
	newDeps := make([]interface{}, 0, len(depList))
	found := false

	for _, dep := range depList {
		depName := ""
		if str, ok := dep.(string); ok {
			depName = str
		} else if obj, ok := dep.(map[string]interface{}); ok {
			if n, ok := obj["name"].(string); ok {
				depName = n
			}
		}

		if depName == name {
			found = true
			continue
		}
		newDeps = append(newDeps, dep)
	}

	if !found {
		return fmt.Errorf("dependency %s not found in vcpkg.json", name)
	}

	// Update manifest
	manifest["dependencies"] = newDeps

	// Write back
	newData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode vcpkg.json: %w", err)
	}

	if err := os.WriteFile("vcpkg.json", newData, 0644); err != nil {
		return fmt.Errorf("failed to write vcpkg.json: %w", err)
	}

	fmt.Printf("%s✓ Removed %s from vcpkg.json%s\n", colors.Green, name, colors.Reset)
	return nil
}

func (b *Builder) ListDependencies(ctx context.Context) ([]build.Dependency, error) {
	// Read vcpkg.json
	data, err := os.ReadFile("vcpkg.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No vcpkg.json means no dependencies
		}
		return nil, fmt.Errorf("failed to read vcpkg.json: %w", err)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse vcpkg.json: %w", err)
	}

	depsRaw, ok := manifest["dependencies"]
	if !ok {
		return nil, nil
	}

	depList, ok := depsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid dependencies format in vcpkg.json")
	}

	var deps []build.Dependency
	for _, dep := range depList {
		var name, version string
		if str, ok := dep.(string); ok {
			name = str
		} else if obj, ok := dep.(map[string]interface{}); ok {
			if n, ok := obj["name"].(string); ok {
				name = n
			}
			if v, ok := obj["version"].(string); ok {
				version = v
			}
		}
		if name != "" {
			deps = append(deps, build.Dependency{
				Name:    name,
				Version: version,
			})
		}
	}

	return deps, nil
}

func (b *Builder) SearchDependencies(ctx context.Context, query string) ([]build.Dependency, error) {
	// Get vcpkg path
	vcpkgPath, err := b.GetPath()
	if err != nil {
		return nil, err
	}

	// Use vcpkg search command
	cmd := exec.Command(vcpkgPath, "search", query)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("vcpkg search failed: %w", err)
	}

	var deps []build.Dependency
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "The result may be outdated") {
			continue
		}
		// Parse format: "name     version     description"
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			name := parts[0]
			version := ""
			description := ""

			// Try to extract version and description
			if len(parts) >= 2 {
				// Simple heuristic: second column is version
				version = parts[1]
			}
			if len(parts) >= 3 {
				description = strings.Join(parts[2:], " ")
			}

			deps = append(deps, build.Dependency{
				Name:        name,
				Version:     version,
				Description: description,
			})
		}
	}

	return deps, nil
}

func (b *Builder) Name() string {
	return "vcpkg"
}

func (b *Builder) DependencyInfo(ctx context.Context, name string) (*build.DependencyInfo, error) {
	// Use vcpkg x-package-info command
	// Format: vcpkg x-package-info <name> --x-json
	if err := b.ensureConfig(); err != nil {
		return nil, err
	}
	cmd := exec.Command(b.globalConfig.VcpkgRoot+"/vcpkg", "x-package-info", name, "--x-json")
	if runtime.GOOS == "windows" {
		cmd.Path += ".exe"
	}

	// Better: Use builder's GetPath
	vcpkgPath, err := b.GetPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get vcpkg path: %w", err)
	}
	cmd = exec.Command(vcpkgPath, "x-package-info", name, "--x-json")

	output, err := cmd.Output()
	// vcpkg x-package-info returns exit code 1 even on success sometimes/wait, check if valid JSON output
	if len(output) == 0 && err != nil {
		return nil, fmt.Errorf("failed to get info for %s: %w", name, err)
	}

	// Parse JSON output
	// Structure matches what was in cli/info.go
	type PackageInfoResult struct {
		Name         string `json:"name"`
		Version      string `json:"version-semver"`
		VersionDate  string `json:"version-date"`
		VersionStr   string `json:"version-string"`
		Description  any    `json:"description"` // string or []string
		Homepage     string `json:"homepage"`
		License      string `json:"license"`
		Dependencies []any  `json:"dependencies"`
	}

	type PackageInfoResponse struct {
		Results map[string]PackageInfoResult `json:"results"`
	}

	// Find the JSON part
	jsonStart := strings.Index(string(output), "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("no package info found for %s", name)
	}
	jsonData := output[jsonStart:]

	var resp PackageInfoResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse package info: %w", err)
	}

	result, ok := resp.Results[name]
	if !ok {
		return nil, fmt.Errorf("package %s not found in results", name)
	}

	// Extract info
	info := &build.DependencyInfo{
		Name:     result.Name,
		Homepage: result.Homepage,
		License:  result.License,
	}

	// Version logic
	info.Version = result.Version
	if info.Version == "" {
		info.Version = result.VersionDate
	}
	if info.Version == "" {
		info.Version = result.VersionStr
	}

	// Description logic
	switch desc := result.Description.(type) {
	case string:
		info.Description = desc
	case []interface{}:
		var lines []string
		for _, d := range desc {
			if s, ok := d.(string); ok {
				lines = append(lines, s)
			}
		}
		info.Description = strings.Join(lines, "\n")
	}

	// Dependencies logic
	for _, dep := range result.Dependencies {
		switch d := dep.(type) {
		case string:
			info.Dependencies = append(info.Dependencies, d)
		case map[string]interface{}:
			if n, ok := d["name"].(string); ok {
				info.Dependencies = append(info.Dependencies, n)
			}
		}
	}

	return info, nil
}

var _ build.BuildSystem = (*Builder)(nil)

func (b *Builder) ListTargets(ctx context.Context) ([]string, error) {
	// Look for any configured build directory in .cache/native
	cacheDir := filepath.Join(".cache", "native")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no build directory found. Run 'cpx build' first")
		}
		return nil, fmt.Errorf("failed to read build cache directory: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() && e.Name() != "vcpkg_installed" {
			bDir := filepath.Join(cacheDir, e.Name())
			targets, err := b.listTargetsInDir(bDir)
			if err == nil && len(targets) > 0 {
				return targets, nil
			}
		}
	}

	return nil, fmt.Errorf("no configured build directory found with targets. Run 'cpx build' first")
}

// listTargetsInDir lists user-defined targets in a specific build directory.
func (b *Builder) listTargetsInDir(bDir string) ([]string, error) {
	// Check for Ninja build
	if _, err := os.Stat(filepath.Join(bDir, "build.ninja")); err == nil {
		// Use ninja -t targets for complete target info
		cmd := exec.Command("ninja", "-C", bDir, "-t", "targets", "all")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, err
		}

		// Parse and filter output to show only user targets
		lines := strings.Split(string(output), "\n")
		var userTargets []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if targetName, isUser := parseNinjaTarget(line); isUser {
				userTargets = append(userTargets, targetName)
			}
		}
		return userTargets, nil
	}

	// Fallback for Make builds
	if isMakefile(filepath.Join(bDir, "Makefile")) {
		cmd := exec.Command("cmake", "--build", bDir, "--target", "help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, err
		}

		lines := strings.Split(string(output), "\n")
		var userTargets []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if targetName, isUser := parseMakeTarget(line); isUser {
				userTargets = append(userTargets, targetName)
			}
		}
		return userTargets, nil
	}

	return nil, fmt.Errorf("no build system found in %s", bDir)
}

func isMakefile(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseNinjaTarget parses a line from ninja -t targets output
func parseNinjaTarget(line string) (string, bool) {
	if line == "" {
		return "", false
	}

	// Parse "target_name: target_type" format
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", false
	}

	targetName := strings.TrimSpace(parts[0])
	targetType := strings.ToUpper(strings.TrimSpace(parts[1]))

	// Skip empty target names
	if targetName == "" {
		return "", false
	}

	// Filter out paths (file-based targets)
	if strings.Contains(targetName, "/") {
		return "", false
	}

	// Detect user-defined targets by their linker type
	isExecutable := strings.Contains(targetType, "EXECUTABLE_LINKER")
	isLibrary := strings.Contains(targetType, "LIBRARY_LINKER")

	if isExecutable || isLibrary {
		return targetName, true
	}

	return "", false
}

// parseMakeTarget parses a line from cmake --build --target help output for Makefile builds
func parseMakeTarget(line string) (string, bool) {
	if line == "" {
		return "", false
	}

	// Make output format varies, but typically lists targets line by line
	// Skip known internal targets
	internalTargets := map[string]bool{
		"all":                     true,
		"clean":                   true,
		"help":                    true,
		"install":                 true,
		"test":                    true,
		"package":                 true,
		"edit_cache":              true,
		"rebuild_cache":           true,
		"list_install_components": true,
		"install/local":           true,
		"install/strip":           true,
		"package_source":          true,
	}

	// Parse "target_name: type" or just "target_name" format
	parts := strings.SplitN(line, ":", 2)
	targetName := strings.TrimSpace(parts[0])

	if targetName == "" {
		return "", false
	}

	if internalTargets[targetName] {
		return "", false
	}

	// Filter out paths and file-based targets
	if strings.Contains(targetName, "/") ||
		strings.HasSuffix(targetName, ".cmake") ||
		strings.HasSuffix(targetName, ".txt") {
		return "", false
	}

	// Filter out CTest internal targets
	if strings.HasPrefix(targetName, "Experimental") ||
		strings.HasPrefix(targetName, "Nightly") ||
		strings.HasPrefix(targetName, "Continuous") {
		return "", false
	}

	return targetName, true
}
