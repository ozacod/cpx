// Package meson provides Meson build system integration.
package meson

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ozacod/cpx/internal/build/common"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/templates"
	"github.com/ozacod/cpx/internal/utils/colors"
)

var execCommand = exec.Command

// Builder implements the build.BuildSystem interface for Meson.
type Builder struct{}

func New() *Builder {
	return &Builder{}
}

func (b *Builder) Build(opts build.BuildOptions) error {
	buildDir := "builddir"

	// Determine build type and optimization from flags
	var buildType, optimization, optLabel string

	switch opts.OptLevel {
	case "0":
		buildType = "debug"
		optimization = "0"
		optLabel = "-O0 (debug)"
	case "1":
		buildType = "debugoptimized"
		optimization = "1"
		optLabel = "-O1"
	case "2":
		buildType = "release"
		optimization = "2"
		optLabel = "-O2"
	case "3":
		buildType = "release"
		optimization = "3"
		optLabel = "-O3"
	case "s":
		buildType = "minsize"
		optimization = "s"
		optLabel = "-Os (size)"
	case "fast":
		// Meson doesn't have -Ofast directly, use -O3 with custom flags
		buildType = "release"
		optimization = "3"
		optLabel = "-Ofast"
	default:
		// No explicit opt level, use release/debug
		if opts.Release {
			buildType = "release"
			optimization = "2"
			optLabel = "release"
		} else {
			buildType = "debug"
			optimization = "0"
			optLabel = "debug"
		}
	}

	// Add sanitizer suffix to label
	if opts.Sanitizer != "" {
		optLabel += "+" + opts.Sanitizer
	}

	// Clean if requested
	if opts.Clean {
		if err := b.Clean(build.CleanOptions{All: false}); err != nil {
			return err
		}
	}

	// Check if build directory exists (needs setup)
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		fmt.Printf("%sSetting up Meson build directory [%s]...%s\n", colors.Cyan, optLabel, colors.Reset)
		setupArgs := []string{"setup", buildDir}
		setupArgs = append(setupArgs, "--buildtype="+buildType)
		setupArgs = append(setupArgs, "--optimization="+optimization)
		if opts.OptLevel == "fast" {
			// Add -ffast-math for -Ofast equivalent
			setupArgs = append(setupArgs, "-Dc_args=-ffast-math", "-Dcpp_args=-ffast-math")
		}
		setupCmd := execCommand("meson", setupArgs...)
		setupCmd.Stdout = os.Stdout
		setupCmd.Stderr = os.Stderr
		if err := setupCmd.Run(); err != nil {
			return fmt.Errorf("meson setup failed: %w", err)
		}
	} else {
		// Build directory exists, reconfigure if optimization changed
		fmt.Printf("%sReconfiguring Meson [%s]...%s\n", colors.Cyan, optLabel, colors.Reset)
		reconfigArgs := []string{"configure", buildDir}
		reconfigArgs = append(reconfigArgs, "--buildtype="+buildType)
		reconfigArgs = append(reconfigArgs, "--optimization="+optimization)
		if opts.OptLevel == "fast" {
			reconfigArgs = append(reconfigArgs, "-Dc_args=-ffast-math", "-Dcpp_args=-ffast-math")
		}
		reconfigCmd := execCommand("meson", reconfigArgs...)
		reconfigCmd.Stdout = os.Stdout
		reconfigCmd.Stderr = os.Stderr
		// Ignore reconfigure errors - may fail if no changes needed
		_ = reconfigCmd.Run()
	}

	// Build
	fmt.Printf("%sBuilding with Meson...%s\n", colors.Cyan, colors.Reset)
	compileArgs := []string{"compile", "-C", buildDir}
	if opts.Target != "" {
		compileArgs = append(compileArgs, opts.Target)
	}
	if opts.Verbose {
		compileArgs = append(compileArgs, "-v")
	}
	buildCmd := execCommand("meson", compileArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("meson compile failed: %w", err)
	}

	// Determine output directory based on config
	outDirName := build.GetOutputDir(opts.Release, opts.OptLevel, opts.Sanitizer)
	outputDir := filepath.Join(".bin", "native", outDirName)

	// Copy artifacts to output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("%sCopying artifacts to %s/...%s\n", colors.Cyan, outputDir, colors.Reset)
	copyCmd := execCommand("bash", "-c", fmt.Sprintf(`
		# Meson places executables in subdirectories (src/, bench/, etc.)
		# Search in builddir/src/ first (main executables)
		if [ -d "builddir/src" ]; then
			find builddir/src -maxdepth 1 -type f -perm +111 ! -name "*.p" ! -name "*_test" -exec cp {} %[1]s/ \; 2>/dev/null || true
		fi

		# Also check builddir root for executables
		find builddir -maxdepth 1 -type f -perm +111 ! -name "*.p" ! -name "*_test" -exec cp {} %[1]s/ \; 2>/dev/null || true

		# Copy libraries from builddir and subdirectories
		find builddir -maxdepth 2 -type f \( -name "*.a" -o -name "*.so" -o -name "*.dylib" \) -exec cp {} %[1]s/ \; 2>/dev/null || true

		# List what was copied
		ls %[1]s/ 2>/dev/null || true
	`, outputDir))
	copyCmd.Stdout = os.Stdout
	copyCmd.Stderr = os.Stderr
	_ = copyCmd.Run()

	fmt.Printf("%s✓ Build successful%s\n", colors.Green, colors.Reset)
	fmt.Printf("  Artifacts in: %s/\n", outputDir)
	return nil
}

func (b *Builder) Test(opts build.TestOptions) error {
	fmt.Printf("%sRunning Meson tests...%s\n", colors.Cyan, colors.Reset)

	// Ensure builddir exists - need build first
	if _, err := os.Stat("builddir"); os.IsNotExist(err) {
		if err := b.Build(build.BuildOptions{Verbose: opts.Verbose}); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
	}

	mesonArgs := []string{"test", "-C", "builddir"}

	// Exclude subproject tests (google-benchmark, gtest, etc.)
	// Only run tests from the main project
	mesonArgs = append(mesonArgs, "--no-suite", "google-benchmark")
	mesonArgs = append(mesonArgs, "--no-suite", "gtest")
	mesonArgs = append(mesonArgs, "--no-suite", "gmock")
	mesonArgs = append(mesonArgs, "--no-suite", "catch2")

	if opts.Verbose {
		mesonArgs = append(mesonArgs, "-v")
	} else {
		mesonArgs = append(mesonArgs, "--quiet")
	}

	if opts.Filter != "" {
		mesonArgs = append(mesonArgs, opts.Filter)
	}

	testCmd := execCommand("meson", mesonArgs...)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("meson test failed: %w", err)
	}

	fmt.Printf("%s✓ Tests passed%s\n", colors.Green, colors.Reset)
	return nil
}

func (b *Builder) Run(opts build.RunOptions) error {
	// Ensure project is built first
	if err := b.Build(build.BuildOptions{
		Release:   opts.Release,
		OptLevel:  opts.OptLevel,
		Sanitizer: opts.Sanitizer,
		Target:    opts.Target,
		Verbose:   opts.Verbose,
	}); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Find executable to run
	var exePath string
	if opts.Target != "" {
		// Try in src/ subdirectory first, then builddir root
		srcPath := filepath.Join("builddir", "src", opts.Target)
		if _, err := os.Stat(srcPath); err == nil {
			exePath = srcPath
		} else {
			exePath = filepath.Join("builddir", opts.Target)
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

	fmt.Printf("%sRunning %s...%s\n", colors.Cyan, exePath, colors.Reset)
	runCmd := execCommand(exePath, opts.Args...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	return runCmd.Run()
}

func (b *Builder) Bench(opts build.BenchOptions) error {
	fmt.Printf("%sRunning Meson benchmarks...%s\n", colors.Cyan, colors.Reset)

	// Ensure builddir exists
	if _, err := os.Stat("builddir"); os.IsNotExist(err) {
		if err := b.Build(build.BuildOptions{Verbose: opts.Verbose}); err != nil {
			return fmt.Errorf("build failed: %w", err)
		}
	}

	// Find benchmark executable
	var benchPath string
	if opts.Target != "" {
		// Try in bench/ subdirectory first, then builddir root
		benchDir := filepath.Join("builddir", "bench", opts.Target)
		if _, err := os.Stat(benchDir); err == nil {
			benchPath = benchDir
		} else {
			benchPath = filepath.Join("builddir", opts.Target)
		}
	} else {
		// Look for *_bench executables in builddir/bench/ first
		searchDirs := []string{filepath.Join("builddir", "bench"), "builddir"}
		for _, dir := range searchDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), "_bench") {
					benchPath = filepath.Join(dir, entry.Name())
					break
				}
			}
			if benchPath != "" {
				break
			}
		}
	}

	if benchPath == "" {
		return fmt.Errorf("no benchmark executable found\n  hint: use --target to specify the benchmark")
	}

	fmt.Printf("  Running: %s\n", benchPath)

	benchCmd := execCommand(benchPath)
	benchCmd.Stdout = os.Stdout
	benchCmd.Stderr = os.Stderr

	if err := benchCmd.Run(); err != nil {
		return fmt.Errorf("benchmark failed: %w", err)
	}

	fmt.Printf("%s✓ Benchmarks complete%s\n", colors.Green, colors.Reset)
	return nil
}

func (b *Builder) Clean(opts build.CleanOptions) error {
	fmt.Printf("%sCleaning Meson project...%s\n", colors.Cyan, colors.Reset)

	// Remove builddir
	common.RemoveDir("builddir")

	// Remove common build output directory
	common.RemoveDir("build")

	if opts.All {
		// Remove additional Meson artifacts
		common.RemoveDir("subprojects/packagecache")

		// Remove build-* directories
		common.RemoveDirsMatchingPattern("build-*", true)
	}

	fmt.Printf("%s✓ Meson project cleaned%s\n", colors.Green, colors.Reset)
	return nil
}

func (b *Builder) AddDependency(name string, _ string) error {
	fmt.Printf("%sInstalling wrap for %s...%s\n", colors.Cyan, name, colors.Reset)

	// Create subprojects dir if it doesn't exist
	if err := os.MkdirAll("subprojects", 0755); err != nil {
		return fmt.Errorf("failed to create subprojects directory: %w", err)
	}

	// Run: meson wrap install <name>
	cmd := execCommand("meson", "wrap", "install", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install wrap for %s: %w", name, err)
	}

	fmt.Printf("%s✓ Added %s%s\n", colors.Green, name, colors.Reset)

	// Print usage info
	fmt.Printf("\n%sUSAGE INFO FOR %s:%s\n", colors.Cyan, name, colors.Reset)
	fmt.Printf("Add this to your meson.build:\n\n")
	fmt.Printf("  %s_dep = dependency('%s')\n\n", name, name)
	fmt.Printf("Then link it to your target:\n\n")
	fmt.Printf("  executable(..., dependencies : %s_dep)\n\n", name)
	fmt.Printf("%s📦 Find more info at:%s\n", colors.Cyan, colors.Reset)
	fmt.Printf("   https://wrapdb.mesonbuild.com/\n\n")

	return nil
}

func (b *Builder) RemoveDependency(name string) error {
	// Remove the wrap file from subprojects
	wrapFile := filepath.Join("subprojects", name+".wrap")
	if _, err := os.Stat(wrapFile); os.IsNotExist(err) {
		return fmt.Errorf("wrap file not found: %s", wrapFile)
	}

	if err := os.Remove(wrapFile); err != nil {
		return fmt.Errorf("failed to remove %s: %w", wrapFile, err)
	}

	// Also try to remove the extracted directory
	extractedDir := filepath.Join("subprojects", name)
	if _, err := os.Stat(extractedDir); err == nil {
		os.RemoveAll(extractedDir)
	}

	fmt.Printf("%s✓ Removed %s%s\n", colors.Green, name, colors.Reset)
	return nil
}

func (b *Builder) ListDependencies() ([]build.Dependency, error) {
	subprojectsDir := "subprojects"
	entries, err := os.ReadDir(subprojectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No subprojects directory means no dependencies
		}
		return nil, fmt.Errorf("failed to read subprojects directory: %w", err)
	}

	var deps []build.Dependency
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".wrap") {
			depName := strings.TrimSuffix(name, ".wrap")
			deps = append(deps, build.Dependency{
				Name:    depName,
				Version: "", // Wrap files don't always have version info easily accessible
			})
		}
	}

	return deps, nil
}

func (b *Builder) SearchDependencies(_ string) ([]build.Dependency, error) {
	// WrapDB search would require HTTP calls to wrapdb.mesonbuild.com
	// For now, return an error indicating this is not implemented
	return nil, fmt.Errorf("SearchDependencies not implemented for Meson - use https://wrapdb.mesonbuild.com to search")
}

func (b *Builder) Name() string {
	return "meson"
}

func (b *Builder) DependencyInfo(_ string) (*build.DependencyInfo, error) {
	return nil, fmt.Errorf("dependency info not implemented for Meson")
}

func (b *Builder) ListTargets() ([]string, error) {
	buildDir := "builddir"
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("build directory '%s' does not exist. Run 'cpx build' first", buildDir)
	}

	// Use meson introspect to get targets as JSON
	cmd := execCommand("meson", "introspect", "--targets", buildDir)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("meson introspect failed: %w", err)
	}

	type MesonTarget struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	var targets []MesonTarget
	if err := json.Unmarshal(output, &targets); err != nil {
		return nil, fmt.Errorf("failed to parse meson targets: %w", err)
	}

	var result []string
	for _, t := range targets {
		result = append(result, fmt.Sprintf("%s (%s)", t.Name, t.Type))
	}

	return result, nil
}

func (b *Builder) GenerateGitignore(projectPath string) error {
	gitignore := templates.GenerateMesonGitignore()
	if err := os.WriteFile(filepath.Join(projectPath, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}
	return nil
}

func (b *Builder) GenerateBuildSrc(projectPath string, config build.InitConfig) error {
	// Generate meson.build (root)
	mesonBuild := templates.GenerateMesonBuildRoot(templates.MesonRootOptions{
		ProjectName:        config.Name,
		IsExe:              !config.IsLibrary,
		CppStandard:        config.CppStandard,
		TestFramework:      config.TestFramework,
		BenchmarkFramework: config.Benchmark,
	})
	if err := os.WriteFile(filepath.Join(projectPath, "meson.build"), []byte(mesonBuild), 0644); err != nil {
		return fmt.Errorf("failed to write meson.build: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(projectPath, "src"), 0755); err != nil {
		return fmt.Errorf("failed to create src directory: %w", err)
	}

	// Generate src/meson.build
	srcMeson := templates.GenerateMesonBuildSrc(config.Name, !config.IsLibrary)
	if err := os.WriteFile(filepath.Join(projectPath, "src/meson.build"), []byte(srcMeson), 0644); err != nil {
		return fmt.Errorf("failed to write src/meson.build: %w", err)
	}

	// Generate meson_options.txt
	mesonOptions := templates.GenerateMesonOptions()
	if err := os.WriteFile(filepath.Join(projectPath, "meson_options.txt"), []byte(mesonOptions), 0644); err != nil {
		return fmt.Errorf("failed to write meson_options.txt: %w", err)
	}

	// Create subprojects directory
	if err := os.MkdirAll(filepath.Join(projectPath, "subprojects"), 0755); err != nil {
		return fmt.Errorf("failed to create subprojects directory: %w", err)
	}

	// Check and install wraps
	if config.TestFramework != "" && config.TestFramework != "none" {
		wrapName := ""
		switch config.TestFramework {
		case "googletest":
			wrapName = "gtest"
		case "catch2":
			wrapName = "catch2"
		case "doctest":
			wrapName = "doctest"
		}
		if wrapName != "" {
			if err := b.downloadWrap(projectPath, wrapName); err != nil {
				fmt.Printf("%sWarning: could not download %s wrap: %v%s\n", colors.Yellow, wrapName, err, colors.Reset)
			}
		}
	}

	if config.Benchmark != "" && config.Benchmark != "none" {
		wrapName := ""
		switch config.Benchmark {
		case "google-benchmark":
			wrapName = "google-benchmark"
		case "catch2-benchmark":
			wrapName = "catch2"
		}
		if wrapName != "" {
			if err := b.downloadWrap(projectPath, wrapName); err != nil {
				fmt.Printf("%sWarning: could not download %s wrap: %v%s\n", colors.Yellow, wrapName, err, colors.Reset)
			}
		}
	}

	return nil
}

func (b *Builder) GenerateBuildTest(projectPath string, config build.InitConfig) error {
	if config.TestFramework == "" || config.TestFramework == "none" {
		return nil
	}

	if err := os.MkdirAll(filepath.Join(projectPath, "tests"), 0755); err != nil {
		return fmt.Errorf("failed to create tests directory: %w", err)
	}

	// Generate tests/meson.build
	testsMeson := templates.GenerateMesonBuildTests(config.Name, config.TestFramework)
	if err := os.WriteFile(filepath.Join(projectPath, "tests/meson.build"), []byte(testsMeson), 0644); err != nil {
		return fmt.Errorf("failed to write tests/meson.build: %w", err)
	}
	return nil
}

func (b *Builder) GenerateBuildBench(projectPath string, config build.InitConfig) error {
	if config.Benchmark == "" || config.Benchmark == "none" {
		return nil
	}

	if err := os.MkdirAll(filepath.Join(projectPath, "bench"), 0755); err != nil {
		return fmt.Errorf("failed to create bench directory: %w", err)
	}

	// Generate bench/meson.build
	benchMeson := templates.GenerateMesonBuildBench(config.Name, config.Benchmark)
	if err := os.WriteFile(filepath.Join(projectPath, "bench/meson.build"), []byte(benchMeson), 0644); err != nil {
		return fmt.Errorf("failed to write bench/meson.build: %w", err)
	}
	return nil
}

func (b *Builder) downloadWrap(projectPath, wrapName string) error {
	// Ensure meson is available (already checked usually)

	cmd := execCommand("meson", "wrap", "install", wrapName)
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("meson wrap install failed for %s: %w", wrapName, err)
	}
	fmt.Printf("  Installed %s.wrap\n", wrapName)
	return nil
}

var _ build.BuildSystem = (*Builder)(nil)
