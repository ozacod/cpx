// Package bazel provides Bazel build system integration.
package bazel

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ozacod/cpx/internal/build/common"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/templates"
	"github.com/ozacod/cpx/internal/utils/colors"
)

var execCommand = exec.Command

// Builder implements the.BuildSystem interface for Bazel.
type Builder struct {
	bcrPath string // BCR path for lazy initialization
}

type BCRPathFunc func() string

// bcrPathProvider is set by CLI to provide BCR path
var bcrPathProvider BCRPathFunc

func SetBCRPathProvider(fn BCRPathFunc) {
	bcrPathProvider = fn
}

func New() *Builder {
	return &Builder{}
}

type Module struct {
	Name        string   `json:"name"`
	Versions    []string `json:"versions"`
	Homepage    string   `json:"homepage"`
	Maintainers []string `json:"maintainers"`
}

type ModuleMetadata struct {
	Homepage    string `json:"homepage"`
	Maintainers []struct {
		Email  string `json:"email"`
		GitHub string `json:"github"`
		Name   string `json:"name"`
	} `json:"maintainers"`
	Versions       []string          `json:"versions"`
	YankedVersions map[string]string `json:"yanked_versions"`
}

func (b *Builder) SetBCRPath(bcrPath string) {
	b.bcrPath = bcrPath
}

func (b *Builder) ensureBCRPath() error {
	if b.bcrPath != "" {
		return nil
	}

	// Try the global provider
	if bcrPathProvider != nil {
		bcrPath := bcrPathProvider()
		if bcrPath != "" {
			b.bcrPath = bcrPath
			return nil
		}
	}

	return fmt.Errorf("bazel Central Registry not configured\n  hint: run 'cpx config set-bcr-root <path>' or reinstall cpx")
}

func (b *Builder) getModulesDir() string {
	return filepath.Join(b.bcrPath, "modules")
}

func (b *Builder) listModules() ([]string, error) {
	modulesDir := b.getModulesDir()
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read modules directory: %w", err)
	}

	var modules []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			modules = append(modules, entry.Name())
		}
	}
	return modules, nil
}

func (b *Builder) searchModules(query string) ([]Module, error) {
	allModules, err := b.listModules()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []Module

	for _, name := range allModules {
		if strings.Contains(strings.ToLower(name), query) {
			module, err := b.getModule(name)
			if err != nil {
				continue // Skip modules with metadata errors
			}
			results = append(results, *module)
		}
	}

	return results, nil
}

func (b *Builder) getModule(moduleName string) (*Module, error) {
	metadataPath := filepath.Join(b.getModulesDir(), moduleName, "metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("module %s not found: %w", moduleName, err)
	}

	var metadata ModuleMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata for %s: %w", moduleName, err)
	}

	module := &Module{
		Name:     moduleName,
		Versions: metadata.Versions,
		Homepage: metadata.Homepage,
	}

	for _, m := range metadata.Maintainers {
		if m.Name != "" {
			module.Maintainers = append(module.Maintainers, m.Name)
		} else if m.GitHub != "" {
			module.Maintainers = append(module.Maintainers, m.GitHub)
		}
	}

	return module, nil
}

func (b *Builder) getLatestVersion(moduleName string) (string, error) {
	module, err := b.getModule(moduleName)
	if err != nil {
		return "", err
	}

	if len(module.Versions) == 0 {
		return "", fmt.Errorf("no versions available for %s", moduleName)
	}

	// Versions are typically listed in order, last is latest
	return module.Versions[len(module.Versions)-1], nil
}

func (b *Builder) Build(opts build.BuildOptions) error {
	// Clean if requested
	if opts.Clean {
		if err := b.Clean(build.CleanOptions{All: false}); err != nil {
			return err
		}
	}

	// Build args
	bazelArgs := []string{"build"}

	// Handle optimization level - optLevel takes precedence over release flag
	var optLabel string
	switch opts.OptLevel {
	case "0":
		bazelArgs = append(bazelArgs, "--copt=-O0", "-c", "dbg")
		optLabel = "-O0 (debug)"
	case "1":
		bazelArgs = append(bazelArgs, "--copt=-O1", "-c", "opt")
		optLabel = "-O1"
	case "2":
		bazelArgs = append(bazelArgs, "--copt=-O2", "-c", "opt")
		optLabel = "-O2"
	case "3":
		bazelArgs = append(bazelArgs, "--copt=-O3", "-c", "opt")
		optLabel = "-O3"
	case "s":
		bazelArgs = append(bazelArgs, "--copt=-Os", "-c", "opt")
		optLabel = "-Os (size)"
	case "fast":
		bazelArgs = append(bazelArgs, "--copt=-Ofast", "-c", "opt")
		optLabel = "-Ofast"
	default:
		// No explicit opt level, use release/debug config
		if opts.Release {
			bazelArgs = append(bazelArgs, "--config=release")
			optLabel = "release"
		} else {
			bazelArgs = append(bazelArgs, "--config=debug")
			optLabel = "debug"
		}
	}

	// Add sanitizer flags
	if opts.Sanitizer != "" {
		switch opts.Sanitizer {
		case "asan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=address", "--copt=-fno-omit-frame-pointer",
				"--linkopt=-fsanitize=address")
			optLabel += "+asan"
		case "tsan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=thread", "--linkopt=-fsanitize=thread")
			optLabel += "+tsan"
		case "msan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=memory", "--copt=-fno-omit-frame-pointer",
				"--linkopt=-fsanitize=memory")
			optLabel += "+msan"
		case "ubsan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=undefined", "--linkopt=-fsanitize=undefined")
			optLabel += "+ubsan"
		}
	}

	// Add target or default to //...
	if opts.Target != "" {
		bazelArgs = append(bazelArgs, opts.Target)
	} else {
		bazelArgs = append(bazelArgs, "//...")
	}

	fmt.Printf("%sBuilding with Bazel [%s]...%s\n", colors.Cyan, optLabel, colors.Reset)
	if opts.Verbose {
		fmt.Printf("  Running: bazel %v\n", bazelArgs)
	} else {
		// Suppress progress bars for cleaner output (like vcpkg)
		// Use hidden symlinks (.bazel-bin, .bazel-out, etc.)
		bazelArgs = append(bazelArgs, "--noshow_progress", "--symlink_prefix=.bazel-")
	}

	buildCmd := execCommand("bazel", bazelArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("bazel build failed: %w", err)
	}

	// Determine output directory based on config
	outDirName := build.GetOutputDir(opts.Release, opts.OptLevel, opts.Sanitizer)
	outputDir := filepath.Join(common.NativeOutputDir(), outDirName)

	// Copy artifacts to build/<config>/ directory
	// Remove existing build artifacts for this config first
	if err := os.RemoveAll(outputDir); err != nil {
		fmt.Printf("Warning: failed to remove %s: %v\n", outputDir, err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Copy executables and libraries from bazel-bin to build/<config>/
	fmt.Printf("%sCopying artifacts to %s/...%s\n", colors.Cyan, outputDir, colors.Reset)

	// Create a script to copy with the correct output directory variable
	script := fmt.Sprintf(`
		# Find the bazel-bin symlink (prefer hidden .bazel-bin)
		BAZEL_BIN=""
		if [ -L ".bazel-bin" ] || [ -d ".bazel-bin" ]; then
			BAZEL_BIN=".bazel-bin"
		elif [ -L "bazel-bin" ] || [ -d "bazel-bin" ]; then
			BAZEL_BIN="bazel-bin"
		elif [ -L ".bin" ] || [ -d ".bin" ]; then
			BAZEL_BIN=".bin"
		fi

		if [ -z "$BAZEL_BIN" ]; then
			echo "No bazel-bin found"
			exit 0
		fi

		# Copy executables from src/ directory (where cc_binary targets are placed)
		find -L "$BAZEL_BIN/src" -maxdepth 1 -type f -perm +111 ! -name "*.params" ! -name "*.sh" ! -name "*.cppmap" ! -name "*.repo_mapping" ! -name "*runfiles*" ! -name "*.d" -exec cp -f {} %[1]s/ \; 2>/dev/null || true

		# Also copy from root of bazel-bin (for root aliases)
		find -L "$BAZEL_BIN" -maxdepth 1 -type f -perm +111 ! -name "*.params" ! -name "*.sh" ! -name "*.cppmap" ! -name "*.repo_mapping" ! -name "*runfiles*" ! -name "*.d" -exec cp -f {} %[1]s/ \; 2>/dev/null || true

		# Copy libraries from src/
		find -L "$BAZEL_BIN/src" -maxdepth 1 -type f \( -name "*.a" -o -name "*.so" -o -name "*.dylib" \) -exec cp -f {} %[1]s/ \; 2>/dev/null || true

		# Make copied files writable (Bazel creates read-only files)
		chmod -R u+w %[1]s/ 2>/dev/null || true

		# List what was copied
		ls %[1]s/ 2>/dev/null || true
	`, outputDir)

	copyCmd := execCommand("bash", "-c", script)
	copyCmd.Stdout = os.Stdout
	copyCmd.Stderr = os.Stderr
	_ = copyCmd.Run() // Ignore errors - may have no artifacts

	fmt.Printf("%s✓ Build successful%s\n", colors.Green, colors.Reset)
	fmt.Printf("  Artifacts in: %s/\n", outputDir)
	return nil
}

func (b *Builder) Test(opts build.TestOptions) error {
	fmt.Printf("%sRunning Bazel tests...%s\n", colors.Cyan, colors.Reset)

	bazelArgs := []string{"test"}

	// Add filter if provided (bazel target pattern)
	if opts.Filter != "" {
		bazelArgs = append(bazelArgs, opts.Filter)
	} else {
		bazelArgs = append(bazelArgs, "//...")
	}

	// Add verbose flag
	if opts.Verbose {
		bazelArgs = append(bazelArgs, "--test_output=all")
	} else {
		bazelArgs = append(bazelArgs, "--test_output=errors")
		// Use hidden symlinks (.bazel-bin, .bazel-out, etc.)
		bazelArgs = append(bazelArgs, "--noshow_progress", "--symlink_prefix=.bazel-")
	}

	testCmd := execCommand("bazel", bazelArgs...)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("bazel test failed: %w", err)
	}

	fmt.Printf("%s✓ Tests passed%s\n", colors.Green, colors.Reset)
	return nil
}

func (b *Builder) Run(opts build.RunOptions) error {
	// Build bazel run args
	bazelArgs := []string{"run"}

	// Handle optimization level
	switch opts.OptLevel {
	case "0":
		bazelArgs = append(bazelArgs, "--copt=-O0", "-c", "dbg")
	case "1":
		bazelArgs = append(bazelArgs, "--copt=-O1", "-c", "opt")
	case "2":
		bazelArgs = append(bazelArgs, "--copt=-O2", "-c", "opt")
	case "3":
		bazelArgs = append(bazelArgs, "--copt=-O3", "-c", "opt")
	case "s":
		bazelArgs = append(bazelArgs, "--copt=-Os", "-c", "opt")
	case "fast":
		bazelArgs = append(bazelArgs, "--copt=-Ofast", "-c", "opt")
	default:
		if opts.Release {
			bazelArgs = append(bazelArgs, "--config=release")
		} else {
			bazelArgs = append(bazelArgs, "--config=debug")
		}
	}

	// Add sanitizer flags
	if opts.Sanitizer != "" {
		switch opts.Sanitizer {
		case "asan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=address", "--copt=-fno-omit-frame-pointer",
				"--linkopt=-fsanitize=address")
		case "tsan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=thread", "--linkopt=-fsanitize=thread")
		case "msan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=memory", "--copt=-fno-omit-frame-pointer",
				"--linkopt=-fsanitize=memory")
		case "ubsan":
			bazelArgs = append(bazelArgs, "--copt=-fsanitize=undefined", "--linkopt=-fsanitize=undefined")
		}
	}

	// Add target or try to find one
	if opts.Target != "" {
		target := opts.Target
		if !strings.HasPrefix(target, "//") && !strings.HasPrefix(target, ":") {
			target = "//:" + target
		}
		bazelArgs = append(bazelArgs, target)
	} else {
		// Try to find the main target from BUILD.bazel
		mainTarget, err := findBazelMainTarget()
		if err != nil {
			return fmt.Errorf("no target specified and could not find main target: %w\n  hint: use --target to specify the target", err)
		}
		bazelArgs = append(bazelArgs, mainTarget)
	}

	// Add -- and user args if present
	if len(opts.Args) > 0 {
		bazelArgs = append(bazelArgs, "--")
		bazelArgs = append(bazelArgs, opts.Args...)
	}

	fmt.Printf("%sRunning with Bazel...%s\n", colors.Cyan, colors.Reset)
	if opts.Verbose {
		fmt.Printf("  Running: bazel %v\n", bazelArgs)
	} else {
		// Use hidden symlinks (.bazel-bin, .bazel-out, etc.)
		bazelArgs = append(bazelArgs, "--noshow_progress", "--symlink_prefix=.bazel-")
	}

	runCmd := execCommand("bazel", bazelArgs...)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	return runCmd.Run()
}

func findBazelMainTarget() (string, error) {
	// Read BUILD.bazel
	content, err := os.ReadFile(common.BazelBuildFile)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", common.BazelBuildFile, err)
	}

	// Look for cc_binary declarations
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name = \"") {
			// Extract target name
			name := strings.TrimPrefix(line, "name = \"")
			name = strings.TrimSuffix(name, "\",")
			name = strings.TrimSuffix(name, "\"")
			// Skip library targets (usually end with _lib)
			if !strings.HasSuffix(name, "_lib") && !strings.HasSuffix(name, "_test") {
				return "//:" + name, nil
			}
		}
	}

	// Fallback: use project directory name
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	projectName := filepath.Base(cwd)
	return "//:" + projectName, nil
}

func (b *Builder) Bench(opts build.BenchOptions) error {
	fmt.Printf("%sRunning Bazel benchmarks...%s\n", colors.Cyan, colors.Reset)

	target := opts.Target
	// If no target specified, query for bench targets
	if target == "" {
		// Query for all cc_binary targets in bench directory
		queryCmd := execCommand("bazel", "query", "kind(cc_binary, //bench:*)")
		output, err := queryCmd.Output()
		if err != nil {
			// Try to find bench target in BUILD.bazel
			target = findBenchTarget()
			if target == "" {
				return fmt.Errorf("no benchmark targets found in //bench")
			}
		} else {
			// Use first target from query
			targets := strings.TrimSpace(string(output))
			if targets == "" {
				return fmt.Errorf("no benchmark targets found in //bench")
			}
			// Take first target
			target = strings.Split(targets, "\n")[0]
		}
	}

	fmt.Printf("  Running: %s\n", target)

	bazelArgs := []string{"run", target}

	if opts.Verbose {
		bazelArgs = append(bazelArgs, "--verbose_failures")
	} else {
		// Use hidden symlinks (.bazel-bin, .bazel-out, etc.)
		bazelArgs = append(bazelArgs, "--noshow_progress", "--symlink_prefix=.bazel-")
	}

	benchCmd := execCommand("bazel", bazelArgs...)
	benchCmd.Stdout = os.Stdout
	benchCmd.Stderr = os.Stderr

	if err := benchCmd.Run(); err != nil {
		return fmt.Errorf("bazel benchmark failed: %w", err)
	}

	fmt.Printf("%s✓ Benchmarks complete%s\n", colors.Green, colors.Reset)
	return nil
}

func findBenchTarget() string {
	data, err := os.ReadFile("bench/BUILD.bazel")
	if err != nil {
		return ""
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name = \"") {
			name := strings.TrimPrefix(line, "name = \"")
			name = strings.TrimSuffix(name, "\",")
			name = strings.TrimSuffix(name, "\"")
			return "//bench:" + name
		}
	}
	return ""
}

func (b *Builder) Clean(opts build.CleanOptions) error {
	fmt.Printf("%sCleaning Bazel project...%s\n", colors.Cyan, colors.Reset)

	// Run bazel clean
	cleanCmd := execCommand("bazel", "clean")
	cleanCmd.Stdout = os.Stdout
	cleanCmd.Stderr = os.Stderr
	if err := cleanCmd.Run(); err != nil {
		fmt.Printf("%s⚠ bazel clean failed (may not be initialized)%s\n", colors.Yellow, colors.Reset)
	} else {
		fmt.Printf("%s✓ Ran bazel clean%s\n", colors.Green, colors.Reset)
	}

	// Remove common build output directory
	common.RemoveDir("build")

	// Remove Bazel symlinks
	// We want to remove .bin, .out, .testlogs which are custom symlinks we might have created
	// And relying on standard bazel clean to remove bazel-*
	bazelSymlinks := []string{".bin", ".out", ".testlogs"}
	for _, symlink := range bazelSymlinks {
		if _, err := os.Lstat(symlink); err == nil {
			fmt.Printf("%s  Removing %s...%s\n", colors.Cyan, symlink, colors.Reset)
			if err := os.RemoveAll(symlink); err != nil {
				fmt.Printf("Warning: failed to remove %s: %v\n", symlink, err)
			}
		}
	}

	// Remove bazel-* symlinks (bazel-bin, bazel-out, bazel-testlogs, bazel-<project>)
	common.RemoveDirsMatchingPattern("bazel-*", false)

	if opts.All {
		// Remove additional Bazel artifacts
		common.RemoveDir(".bazel")
		common.RemoveDir("external")
	}

	fmt.Printf("%s✓ Bazel project cleaned%s\n", colors.Green, colors.Reset)
	return nil
}

func (b *Builder) AddDependency(name string, version string) error {
	// If no version provided, get latest from BCR
	if version == "" {
		// Ensure BCR path is configured
		if err := b.ensureBCRPath(); err != nil {
			return err
		}

		latestVersion, err := b.getLatestVersion(name)
		if err != nil {
			return fmt.Errorf("module '%s' not found in BCR: %w", name, err)
		}
		version = latestVersion
	}

	// Read MODULE.bazel
	content, err := os.ReadFile(common.BazelModuleFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", common.BazelModuleFile, err)
	}

	// Check if dependency already exists
	depPattern := regexp.MustCompile(fmt.Sprintf(`bazel_dep\s*\(\s*name\s*=\s*"%s"`, regexp.QuoteMeta(name)))
	if depPattern.Match(content) {
		// Update existing dependency
		updatePattern := regexp.MustCompile(fmt.Sprintf(`(bazel_dep\s*\(\s*name\s*=\s*"%s"\s*,\s*version\s*=\s*")[^"]*(")\)`, regexp.QuoteMeta(name)))
		newContent := updatePattern.ReplaceAll(content, []byte(fmt.Sprintf(`${1}%s${2})`, version)))
		if err := os.WriteFile(common.BazelModuleFile, newContent, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", common.BazelModuleFile, err)
		}
	} else {
		// Add new dependency at the end
		newDep := fmt.Sprintf("\nbazel_dep(name = \"%s\", version = \"%s\")\n", name, version)
		content = append(content, []byte(newDep)...)
		if err := os.WriteFile(common.BazelModuleFile, content, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", common.BazelModuleFile, err)
		}
	}

	fmt.Printf("%s✓ Added %s@%s to %s%s\n", colors.Green, name, version, common.BazelModuleFile, colors.Reset)

	// Print usage info
	fmt.Printf("\n%sUSAGE INFO FOR %s:%s\n", colors.Cyan, name, colors.Reset)
	fmt.Printf("Add this to your %s:\n\n", common.BazelBuildFile)
	fmt.Printf("  deps = [\"@%s//:<target>\"]\n\n", name)
	fmt.Printf("%s📦 Find more info at:%s\n", colors.Cyan, colors.Reset)
	fmt.Printf("   https://registry.bazel.build/modules/%s\n\n", name)

	return nil
}

func (b *Builder) RemoveDependency(name string) error {
	content, err := os.ReadFile(common.BazelModuleFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", common.BazelModuleFile, err)
	}

	// Remove the dependency line
	pattern := regexp.MustCompile(fmt.Sprintf(`\n?bazel_dep\s*\(\s*name\s*=\s*"%s"[^)]*\)\n?`, regexp.QuoteMeta(name)))
	newContent := pattern.ReplaceAll(content, []byte(""))

	if err := os.WriteFile(common.BazelModuleFile, newContent, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.BazelModuleFile, err)
	}

	fmt.Printf("%s✓ Removed %s from %s%s\n", colors.Green, name, common.BazelModuleFile, colors.Reset)
	return nil
}

func (b *Builder) ListDependencies() ([]build.Dependency, error) {
	content, err := os.ReadFile(common.BazelModuleFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", common.BazelModuleFile, err)
	}

	// Match bazel_dep(name = "xxx", version = "yyy")
	pattern := regexp.MustCompile(`bazel_dep\s*\(\s*name\s*=\s*"([^"]+)"\s*,\s*version\s*=\s*"([^"]+)"\s*\)`)
	matches := pattern.FindAllStringSubmatch(string(content), -1)

	var deps []build.Dependency
	for _, match := range matches {
		if len(match) >= 3 {
			deps = append(deps, build.Dependency{
				Name:    match[1],
				Version: match[2],
			})
		}
	}

	return deps, nil
}

func (b *Builder) SearchDependencies(query string) ([]build.Dependency, error) {
	if err := b.ensureBCRPath(); err != nil {
		return nil, err
	}

	modules, err := b.searchModules(query)
	if err != nil {
		return nil, fmt.Errorf("failed to search BCR: %w", err)
	}

	var deps []build.Dependency
	for _, m := range modules {
		version := ""
		if len(m.Versions) > 0 {
			version = m.Versions[len(m.Versions)-1]
		}
		deps = append(deps, build.Dependency{
			Name:        m.Name,
			Version:     version,
			Description: m.Homepage, // Use homepage as description for now
		})
	}

	return deps, nil
}

func (b *Builder) Name() string {
	return "bazel"
}

func (b *Builder) DependencyInfo(name string) (*build.DependencyInfo, error) {
	// Use BCR client to get module info
	if err := b.ensureBCRPath(); err != nil {
		return nil, err
	}

	module, err := b.getModule(name)
	if err != nil {
		return nil, err
	}

	info := &build.DependencyInfo{
		Name:     module.Name,
		Homepage: module.Homepage,
		// License: module.License, // BCR metadata doesn't strictly have license field easily accessible in all versions, but we can check
	}

	// Get latest version
	if len(module.Versions) > 0 {
		info.Version = module.Versions[len(module.Versions)-1]
	}

	// Description is not always available in BCR metadata in a standard way,
	// often users rely on Homepage or maintainers.
	// We'll leave description empty or use homepage as fallback.
	if info.Description == "" {
		info.Description = info.Homepage
	}

	return info, nil
}

func (b *Builder) ListTargets() ([]string, error) {
	// Query for all targets of type rule
	// We use label_kind to get type info: "kind rule //package:target"
	cmd := execCommand("bazel", "query", "//...", "--output", "label_kind")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("bazel query failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var targets []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "rule_kind rule label"
		// e.g. "cc_binary rule //src:main"
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			kind := parts[0]
			label := parts[len(parts)-1] // last part is label
			// Filter interesting targets
			if strings.HasPrefix(kind, "cc_") {
				targets = append(targets, fmt.Sprintf("%s (%s)", label, kind))
			}
		}
	}

	return targets, nil
}

func (b *Builder) GenerateGitignore(projectPath string) error {
	gitignore := templates.GenerateBazelGitignore()
	if err := os.WriteFile(filepath.Join(projectPath, common.GitignoreFile), []byte(gitignore), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.GitignoreFile, err)
	}
	return nil
}

func (b *Builder) GenerateBuildSrc(projectPath string, config build.InitConfig) error {
	// Generate MODULE.bazel
	moduleBazel := templates.GenerateModuleBazel(config.Name, config.Version, config.TestFramework, config.Benchmark)
	if err := os.WriteFile(filepath.Join(projectPath, common.BazelModuleFile), []byte(moduleBazel), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.BazelModuleFile, err)
	}

	// Generate root BUILD.bazel (aliases)
	buildBazel := templates.GenerateBuildBazelRoot(config.Name, !config.IsLibrary)
	if err := os.WriteFile(filepath.Join(projectPath, common.BazelBuildFile), []byte(buildBazel), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.BazelBuildFile, err)
	}

	if err := os.MkdirAll(filepath.Join(projectPath, "src"), 0755); err != nil {
		return fmt.Errorf("failed to create src directory: %w", err)
	}

	// Generate src/BUILD.bazel
	srcBuild := templates.GenerateBuildBazelSrc(config.Name, !config.IsLibrary)
	if err := os.WriteFile(filepath.Join(projectPath, "src/BUILD.bazel"), []byte(srcBuild), 0644); err != nil {
		return fmt.Errorf("failed to write src/BUILD.bazel: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(projectPath, "include"), 0755); err != nil {
		return fmt.Errorf("failed to create include directory: %w", err)
	}

	// Generate include/BUILD.bazel
	includeBuild := templates.GenerateBuildBazelInclude(config.Name)
	if err := os.WriteFile(filepath.Join(projectPath, "include/BUILD.bazel"), []byte(includeBuild), 0644); err != nil {
		return fmt.Errorf("failed to write include/BUILD.bazel: %w", err)
	}

	// Generate .bazelrc
	bazelrc := templates.GenerateBazelrc(config.CppStandard)
	if err := os.WriteFile(filepath.Join(projectPath, common.BazelRcFile), []byte(bazelrc), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.BazelRcFile, err)
	}

	// Generate .bazelignore
	bazelignore := templates.GenerateBazelignore()
	if err := os.WriteFile(filepath.Join(projectPath, ".bazelignore"), []byte(bazelignore), 0644); err != nil {
		return fmt.Errorf("failed to write .bazelignore: %w", err)
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

	// Generate tests/BUILD.bazel
	testsBuild := templates.GenerateBuildBazelTests(config.Name, config.TestFramework)
	if err := os.WriteFile(filepath.Join(projectPath, "tests/BUILD.bazel"), []byte(testsBuild), 0644); err != nil {
		return fmt.Errorf("failed to write tests/BUILD.bazel: %w", err)
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

	// Generate bench/BUILD.bazel
	benchBuild := templates.GenerateBuildBazelBench(config.Name, config.Benchmark)
	if err := os.WriteFile(filepath.Join(projectPath, "bench/BUILD.bazel"), []byte(benchBuild), 0644); err != nil {
		return fmt.Errorf("failed to write bench/BUILD.bazel: %w", err)
	}
	return nil
}

var _ build.BuildSystem = (*Builder)(nil)
