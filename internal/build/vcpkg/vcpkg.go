// Package vcpkg provides vcpkg build system integration.
package vcpkg

import (
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

	"github.com/ozacod/cpx/internal/build/common"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/config"
	"github.com/ozacod/cpx/internal/templates"
	"github.com/ozacod/cpx/internal/utils"
	"github.com/ozacod/cpx/internal/utils/colors"
)

var execCommand = exec.Command

// VcpkgBuilder implements the build.BuildSystem interface for vcpkg.
type VcpkgBuilder struct {
	globalConfig *config.GlobalConfig
}

func New() *VcpkgBuilder {
	return &VcpkgBuilder{}
}

// ensureConfig ensures the global config is loaded
func (b *VcpkgBuilder) ensureConfig() error {
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

func (b *VcpkgBuilder) SetupEnv() error {
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

func (b *VcpkgBuilder) configureCMake(opts configureOptions) error {
	// Determine absolute path for shared vcpkg_installed directory
	cwd, _ := os.Getwd()
	vcpkgInstalledDir := filepath.Join(cwd, common.NativeCacheDir(), "vcpkg_installed")
	vcpkgInstallArg := "-DVCPKG_INSTALLED_DIR=" + vcpkgInstalledDir

	// Get vcpkg toolchain file path
	vcpkgRoot := os.Getenv("VCPKG_ROOT")
	if vcpkgRoot == "" {
		return fmt.Errorf("VCPKG_ROOT not set. Run: cpx config set-vcpkg-root <path>")
	}
	toolchainFile := filepath.Join(vcpkgRoot, "scripts", "buildsystems", "vcpkg.cmake")

	cmdArgs := []string{
		"-B", opts.buildDir,
		"-G", "Ninja",
		"-DCMAKE_BUILD_TYPE=" + opts.buildType,
		"-DCMAKE_TOOLCHAIN_FILE=" + toolchainFile,
		vcpkgInstallArg,
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
		for i, arg := range cmdArgs {
			if strings.HasPrefix(arg, "-DCMAKE_BUILD_TYPE=") {
				cmdArgs[i] = "-DCMAKE_BUILD_TYPE=Release"
				break
			}
		}
	}

	cmd := execCommand("cmake", cmdArgs...)
	cmd.Env = os.Environ()

	if err := common.RunCMakeConfigure(cmd, opts.verbose); err != nil {
		return fmt.Errorf("cmake configure failed: %w", err)
	}

	return nil
}

func (b *VcpkgBuilder) copyBuildArtifacts(cacheBuildDir, finalBuildDir string) error {
	if err := os.MkdirAll(finalBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create final build dir: %w", err)
	}

	executables, err := common.FindExecutables(cacheBuildDir)
	if err == nil {
		for _, exe := range executables {
			dest := filepath.Join(finalBuildDir, filepath.Base(exe))
			if err := common.CopyAndSign(exe, dest); err != nil {
				utils.PrintError("failed to copy executable %s: %v", filepath.Base(exe), err)
			}
		}
	}

	libraries, err := common.FindLibraries(cacheBuildDir)
	if err == nil {
		for _, lib := range libraries {
			dest := filepath.Join(finalBuildDir, filepath.Base(lib))
			if err := common.CopyAndSign(lib, dest); err != nil {
				utils.PrintError("failed to copy library %s: %v", filepath.Base(lib), err)
			}
		}
	}

	return nil
}

func (b *VcpkgBuilder) GetPath() (string, error) {
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

func (b *VcpkgBuilder) RunCommand(args []string) error {
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

func (b *VcpkgBuilder) GenerateGitignore(projectPath string) error {
	gitignore := templates.GenerateGitignore()
	if err := os.WriteFile(filepath.Join(projectPath, common.GitignoreFile), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.GitignoreFile, err)
	}
	return nil
}

func (b *VcpkgBuilder) GenerateBuildSrc(projectPath string, config build.InitConfig) error {
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
	if err := os.WriteFile(filepath.Join(projectPath, common.CMakeListsFile), []byte(cmakeLists), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.CMakeListsFile, err)
	}

	return nil
}

func (b *VcpkgBuilder) GenerateBuildTest(projectPath string, config build.InitConfig) error {
	if config.TestFramework == "" || config.TestFramework == "none" {
		return nil
	}

	if err := os.MkdirAll(filepath.Join(projectPath, "tests"), 0755); err != nil {
		return fmt.Errorf("failed to create tests directory: %w", err)
	}

	// Generate tests/CMakeLists.txt
	testCMake := templates.GenerateTestCMake(config.Name, config.TestFramework)
	if err := os.WriteFile(filepath.Join(projectPath, "tests", common.CMakeListsFile), []byte(testCMake), 0644); err != nil {
		return fmt.Errorf("failed to write tests/%s: %w", common.CMakeListsFile, err)
	}
	return nil
}

func (b *VcpkgBuilder) GenerateBuildBench(projectPath string, config build.InitConfig) error {
	if config.Benchmark == "" || config.Benchmark == "none" {
		return nil
	}

	if err := os.MkdirAll(filepath.Join(projectPath, string(common.VariantBench)), 0755); err != nil {
		return fmt.Errorf("failed to create bench directory: %w", err)
	}

	// Generate bench/CMakeLists.txt
	benchCMake := templates.GenerateBenchCMake(config.Name, config.Benchmark)
	if err := os.WriteFile(filepath.Join(projectPath, string(common.VariantBench), common.CMakeListsFile), []byte(benchCMake), 0644); err != nil {
		return fmt.Errorf("failed to write bench/%s: %w", common.CMakeListsFile, err)
	}
	return nil
}

func (b *VcpkgBuilder) GenerateReadme(config build.InitConfig) string {
	return templates.GenerateVcpkgReadme(config.Name, config.CppStandard, config.IsLibrary)
}

func (b *VcpkgBuilder) Update() error {
	if err := b.SetupEnv(); err != nil {
		return err
	}

	vcpkgPath, err := b.GetPath()
	if err != nil {
		return fmt.Errorf("vcpkg not configured: %w", err)
	}

	// Check if we're in manifest mode (vcpkg.json exists)
	if _, err := os.Stat(common.VcpkgManifest); err == nil {
		fmt.Printf("%s Checking baseline status...%s\n", colors.Cyan, colors.Reset)
		fmt.Printf("\nIn manifest mode, packages are pinned to a baseline in vcpkg.json.\n")
		fmt.Printf("To update to latest versions, run: %scpx upgrade%s\n\n", colors.Cyan, colors.Reset)

		// Show current dependencies
		deps, err := b.ListDependencies()
		if err == nil && len(deps) > 0 {
			fmt.Printf("Current dependencies:\n")
			for _, dep := range deps {
				if dep.Version != "" {
					fmt.Printf("  %s%s%s @ %s\n", colors.Green, dep.Name, colors.Reset, dep.Version)
				} else {
					fmt.Printf("  %s%s%s\n", colors.Green, dep.Name, colors.Reset)
				}
			}
		}
		return nil
	}

	// Classic mode - use vcpkg update
	fmt.Printf("%s Checking for outdated packages...%s\n", colors.Cyan, colors.Reset)

	cmd := execCommand(vcpkgPath, "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vcpkg update failed: %w", err)
	}

	return nil
}

func (b *VcpkgBuilder) Upgrade() error {
	if err := b.SetupEnv(); err != nil {
		return err
	}

	vcpkgPath, err := b.GetPath()
	if err != nil {
		return fmt.Errorf("vcpkg not configured: %w", err)
	}

	// Check if we're in manifest mode (vcpkg.json exists)
	if _, err := os.Stat(common.VcpkgManifest); err == nil {
		fmt.Printf("%s Updating baseline to latest...%s\n", colors.Cyan, colors.Reset)

		// Use x-update-baseline to update the baseline in vcpkg.json
		cmd := execCommand(vcpkgPath, "x-update-baseline", "--add-initial-baseline")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("vcpkg x-update-baseline failed: %w", err)
		}

		fmt.Printf("\n%s Baseline updated! Run %scpx build%s to install updated packages.%s\n",
			colors.Green, colors.Cyan, colors.Green, colors.Reset)
		return nil
	}

	// Classic mode - use vcpkg upgrade
	fmt.Printf("%s Upgrading packages...%s\n", colors.Cyan, colors.Reset)

	cmd := execCommand(vcpkgPath, "upgrade", "--no-dry-run")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vcpkg upgrade failed: %w", err)
	}

	fmt.Printf("%s Packages upgraded successfully!%s\n", colors.Green, colors.Reset)
	return nil
}

func (b *VcpkgBuilder) Build(opts build.BuildOptions) error {
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
		outDirName = string(common.VariantTest)
	} else if opts.EnableBenchmarks {
		outDirName = string(common.VariantBench)
	} else {
		outDirName = build.GetOutputDir(opts.Release, opts.OptLevel, opts.Sanitizer)
	}

	// Use hidden cache directory for build artifacts
	// .cache/native/<variant>
	cacheBuildDir := filepath.Join(common.NativeCacheDir(), outDirName)
	// Final executables go to .bin/native/<variant>
	finalBuildDir := filepath.Join(common.NativeOutputDir(), outDirName)

	if opts.Clean {
		if opts.Verbose {
			fmt.Printf("%s  Cleaning build directory...%s\n", colors.Cyan, colors.Reset)
		}
		if err := os.RemoveAll(cacheBuildDir); err != nil {
			utils.PrintError("failed to clean cache directory: %v", err)
		}
		if err := os.RemoveAll(finalBuildDir); err != nil {
			utils.PrintError("failed to clean build directory: %v", err)
		}
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

	optLabel := common.GetBuildOptLabel(release, opts.OptLevel, opts.Sanitizer)

	// Customize header based on build type
	buildHeader := buildType
	if opts.EnableTesting {
		buildHeader = buildType + "+tests"
	} else if opts.EnableBenchmarks {
		buildHeader = buildType + "+bench"
	}

	// Configure CMake if needed
	needsConfigure := false
	if _, err := os.Stat(filepath.Join(cacheBuildDir, common.CMakeCacheFile)); os.IsNotExist(err) {
		needsConfigure = true
	}

	// Define steps based on what we need to do
	var stepNames []string
	configureIdx, buildIdx, copyIdx := -1, -1, -1

	if needsConfigure {
		configureIdx = len(stepNames)
		stepNames = append(stepNames, "Configuring")
	}
	buildIdx = len(stepNames)
	stepNames = append(stepNames, "Building")

	// Only add copy step for non-test/bench builds
	if !opts.EnableTesting && !opts.EnableBenchmarks {
		copyIdx = len(stepNames)
		stepNames = append(stepNames, "Copying")
	}

	// Create step progress tracker
	sp := common.NewStepProgress(projectName, buildHeader, optLabel, stepNames, opts.Verbose)

	// Mark steps without parsable progress as indeterminate
	if configureIdx >= 0 {
		sp.SetIndeterminate(configureIdx, true)
	}
	if copyIdx >= 0 {
		sp.SetIndeterminate(copyIdx, true)
	}

	sp.Start()

	buildStart := time.Now()

	if needsConfigure {
		sp.StartStep(configureIdx)

		if err := b.configureCMake(configureOptions{
			buildDir:         cacheBuildDir,
			buildType:        buildType,
			cxxFlags:         cxxFlags,
			linkerFlags:      linkerFlags,
			enableTesting:    opts.EnableTesting,
			enableBenchmarks: opts.EnableBenchmarks,
			verbose:          opts.Verbose,
		}); err != nil {
			sp.FailStep(configureIdx)
			sp.Finish(false)
			return err
		}

		sp.CompleteStep(configureIdx)
	}

	// Build
	sp.StartStep(buildIdx)

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

	if err := common.RunCMakeBuildWithSteps(buildArgs, sp, buildIdx, opts.Verbose); err != nil {
		sp.FailStep(buildIdx)
		sp.Finish(false)
		return fmt.Errorf("build failed: %w", err)
	}

	sp.CompleteStep(buildIdx)

	// Copy artifacts to final build directory (skip for test/bench builds)
	if !opts.EnableTesting && !opts.EnableBenchmarks {
		sp.StartStep(copyIdx)

		if err := b.copyBuildArtifacts(cacheBuildDir, finalBuildDir); err != nil {
			sp.FailStep(copyIdx)
			sp.Finish(false)
			return err
		}

		sp.CompleteStep(copyIdx)
		sp.Finish(true)

		fmt.Printf("%s  ✔ Build complete%s %s[%s]%s\n", colors.Green, colors.Reset, colors.Gray, time.Since(buildStart).Round(10*time.Millisecond), colors.Reset)
		fmt.Printf("  Artifacts in: %s/\n\n", finalBuildDir)
	} else {
		sp.Finish(true)
		fmt.Printf("%s  ✔ Build complete%s %s[%s]%s\n", colors.Green, colors.Reset, colors.Gray, time.Since(buildStart).Round(10*time.Millisecond), colors.Reset)
	}

	return nil
}

func (b *VcpkgBuilder) Test(opts build.TestOptions) error {
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

	if err := b.Build(buildOpts); err != nil {
		return fmt.Errorf("failed to build tests: %w", err)
	}

	// Run tests with CTest
	buildDir := filepath.Join(common.NativeCacheDir(), string(common.VariantTest))

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

func (b *VcpkgBuilder) Run(opts build.RunOptions) error {
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

	if err := b.Build(buildOpts); err != nil {
		return err
	}

	// Determine where artifacts are
	outDirName := build.GetOutputDir(opts.Release, opts.OptLevel, opts.Sanitizer)
	finalBuildDir := filepath.Join(common.NativeOutputDir(), outDirName)

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

func (b *VcpkgBuilder) Bench(opts build.BenchOptions) error {
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

	if err := b.Build(buildOpts); err != nil {
		return fmt.Errorf("failed to build benchmarks: %w", err)
	}

	// Run benchmarks
	buildDir := filepath.Join(common.NativeCacheDir(), string(common.VariantBench))

	fmt.Printf("%s▸ Running benchmarks...%s\n", colors.Cyan, colors.Reset)

	// Find the benchmark executable
	possiblePaths := []string{
		filepath.Join(buildDir, string(common.VariantBench), benchTarget),
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

func (b *VcpkgBuilder) Clean(opts build.CleanOptions) error {
	fmt.Printf("%sCleaning CMake/vcpkg project...%s\n", colors.Cyan, colors.Reset)

	// Remove bin directory (artifacts)
	common.RemoveDir(common.NativeOutputDir())

	// Remove intermediate build directories (keep vcpkg_installed unless --all)
	// We iterate common variants instead of blowing away .cache/native
	variants := []string{
		string(common.VariantDebug), string(common.VariantRelease),
		"O0", "O1", "O2", "O3", "Os", "Ofast",
		string(common.VariantTest), string(common.VariantBench),
	}
	for _, v := range variants {
		common.RemoveDir(filepath.Join(common.NativeCacheDir(), v))
	}

	if opts.All {
		// Clean everything including vcpkg dependencies and CI artifacts
		dirsToRemove := []string{
			common.NativeCacheDir(),
			filepath.Join(common.CacheDir, "ci"),
			filepath.Join(common.OutputDir, "ci"),
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

//TODO: implement version for vcpkg add

func (b *VcpkgBuilder) AddDependency(name string, _ string) error {
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
func (b *VcpkgBuilder) printUsageInfo(pkgName string) {
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/microsoft/vcpkg/master/ports/%s/usage", pkgName))
	if err != nil || resp.StatusCode != 200 {
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			utils.PrintError("error closing response body: %v", err)
		}
	}()

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

func (b *VcpkgBuilder) RemoveDependency(name string) error {
	// Check for vcpkg.json (Manifest mode)
	if _, err := os.Stat(common.VcpkgManifest); err != nil {
		return fmt.Errorf("%s not found - manifest mode required", common.VcpkgManifest)
	}

	// Read manifest
	data, err := os.ReadFile(common.VcpkgManifest)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", common.VcpkgManifest, err)
	}

	// Parse JSON
	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse %s: %w", common.VcpkgManifest, err)
	}

	// Get dependencies
	deps, ok := manifest["dependencies"]
	if !ok {
		return fmt.Errorf("no dependencies found in %s", common.VcpkgManifest)
	}

	depList, ok := deps.([]interface{})
	if !ok {
		return fmt.Errorf("invalid dependencies format in %s", common.VcpkgManifest)
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
		return fmt.Errorf("dependency %s not found in %s", name, common.VcpkgManifest)
	}

	// Update manifest
	manifest["dependencies"] = newDeps

	// Write back
	newData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode %s: %w", common.VcpkgManifest, err)
	}

	if err := os.WriteFile(common.VcpkgManifest, newData, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.VcpkgManifest, err)
	}

	fmt.Printf("%s✓ Removed %s from %s%s\n", colors.Green, name, common.VcpkgManifest, colors.Reset)
	return nil
}

func (b *VcpkgBuilder) ListDependencies() ([]build.Dependency, error) {
	// Read vcpkg.json
	data, err := os.ReadFile(common.VcpkgManifest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No vcpkg.json means no dependencies
		}
		return nil, fmt.Errorf("failed to read %s: %w", common.VcpkgManifest, err)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", common.VcpkgManifest, err)
	}

	depsRaw, ok := manifest["dependencies"]
	if !ok {
		return nil, nil
	}

	depList, ok := depsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid dependencies format in %s", common.VcpkgManifest)
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

func (b *VcpkgBuilder) SearchDependencies(query string) ([]build.Dependency, error) {
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

func (b *VcpkgBuilder) Name() string {
	return "vcpkg"
}

func (b *VcpkgBuilder) DependencyInfo(name string) (*build.DependencyInfo, error) {
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

var _ build.BuildSystem = (*VcpkgBuilder)(nil)

func (b *VcpkgBuilder) ListTargets() ([]string, error) {
	// Look for any configured build directory in .cache/native
	cacheDir := common.NativeCacheDir()
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
func (b *VcpkgBuilder) listTargetsInDir(bDir string) ([]string, error) {
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
