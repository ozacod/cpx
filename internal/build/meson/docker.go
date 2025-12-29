// Package meson provides Meson build system Docker integration.
package meson

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/utils/colors"
)

func (b *Builder) RunDockerBuild(opts build.DockerBuildOptions) error {
	absProjectRoot, err := filepath.Abs(opts.ProjectRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for project root: %w", err)
	}

	absOutputDir, err := filepath.Abs(filepath.Join(opts.ProjectRoot, opts.OutputDir))
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output directory: %w", err)
	}
	// Create output directory on host
	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create persistent build directory
	hostBuildDir := filepath.Join(opts.ProjectRoot, ".cache", "ci", opts.TargetName)
	if err := os.MkdirAll(hostBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}
	absBuildDir, err := filepath.Abs(hostBuildDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for build directory: %w", err)
	}

	// Determine build type
	buildType := "release"
	if opts.BuildType == "Debug" || opts.BuildType == "debug" {
		buildType = "debug"
	}

	// Create subprojects directory
	hostSubprojectsDir := filepath.Join(opts.ProjectRoot, "subprojects")
	if err := os.MkdirAll(hostSubprojectsDir, 0755); err != nil {
		return fmt.Errorf("failed to create subprojects directory: %w", err)
	}
	absSubprojectsDir, err := filepath.Abs(hostSubprojectsDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for subprojects directory: %w", err)
	}

	// Environment exports
	var envExports string
	if len(opts.Env) > 0 {
		envExports = "# User-defined environment variables\n"
		for k, v := range opts.Env {
			envExports += fmt.Sprintf("export %s=\"%s\"\n", k, v)
		}
	}

	// Build Meson arguments
	setupArgs := []string{"--buildtype=" + buildType}
	setupArgs = append(setupArgs, opts.MesonArgs...)

	// Detect project name
	projectName := GetProjectNameFromMesonBuild(opts.ProjectRoot)
	if projectName == "" {
		projectName = filepath.Base(opts.ProjectRoot)
	}

	testSection := ""
	if opts.RunTests {
		testSection = fmt.Sprintf(`
echo "  Running tests..."
meson test -C /tmp/builddir -v "%s:"
`, projectName)
	}

	benchSection := ""
	if opts.RunBenchmarks {
		benchSection = fmt.Sprintf(`
echo "  Running benchmarks..."
meson test -C /tmp/builddir --benchmark -v "%s:" || true
# Also run any manually built benchmark binaries
for bench in $(find /tmp/builddir -maxdepth 2 -type f -executable -name "*_bench" 2>/dev/null); do
    echo "  Running $bench..."
    $bench
done
`, projectName)
	}

	// Handle verbosity
	mesonQuiet := ""
	if !opts.Verbose {
		mesonQuiet = " > /dev/null 2>&1"
	}

	setupEcho := "echo \"  Configuring Meson...\""
	buildEcho := "echo \"  Building...\""
	copyEcho := "echo \"  Copying artifacts...\""
	if !opts.Verbose {
		setupEcho = ":"
		buildEcho = ":"
		copyEcho = ":"
	}

	isVerbose := "false"
	if opts.Verbose {
		isVerbose = "true"
	}

	runSection := ""
	if opts.ExecuteAfterBuild {
		runSection = `
echo "  Running executable..."
cd /output/%[1]s
EXEC=""
# Priority 1: Project name
if [ -f "./%[2]s" ] && [ -x "./%[2]s" ]; then
    EXEC="./%[2]s"
# Priority 2: Toolchain name
elif [ -f "./%[1]s" ] && [ -x "./%[1]s" ]; then
    EXEC="./%[1]s"
else
    # Priority 3: Any executable that doesn't look like an internal file
    for f in $(find . -maxdepth 2 -type f -executable ! -name ".*" ! -name "*.so" ! -name "*.dylib" ! -name "*.a" ! -name "*.p" ! -name "*.ninja" ! -name "*.dat" ! -name "*.txt" ! -path "*/.*" 2>/dev/null); do
        EXEC="$f"
        break
    done
fi
if [ -n "$EXEC" ]; then
    echo "  Executing: $EXEC"
    $EXEC
else
    echo "  No executable found to run"
fi
cd - > /dev/null
`
		runSection = fmt.Sprintf(runSection, opts.TargetName, projectName)
	}
	buildCompleteEcho := "echo \"  Build complete!\""
	if opts.ExecuteAfterBuild {
		buildCompleteEcho = ":"
	}

	// Arguments for fmt.Sprintf in order of appearance (or referenced by index)
	// 1: envExports
	// 2: setupEcho
	// 3: setupArgs
	// 4: mesonQuiet
	// 5: isVerbose
	// 6: buildEcho
	// 7: copyEcho
	// 8: TargetName
	// 9: testSection
	// 10: benchSection
	// 11: runSection
	// 12: buildCompleteEcho
	// 13: projectName
	buildScript := fmt.Sprintf(`#!/bin/bash
set -e
%[1]s
mkdir -p /tmp/builddir
%[2]s
if [ ! -f /tmp/builddir/build.ninja ]; then
    meson setup /tmp/builddir %[3]s%[4]s
else
    if [ "%[5]s" = "true" ]; then echo "  Build directory already configured, skipping setup."; fi
fi
%[6]s
meson compile -C /tmp/builddir%[4]s
%[7]s
mkdir -p /output/%[8]s
# Recursive find excluding internal dirs
find /tmp/builddir -maxdepth 3 -type f -perm /111 ! -path "*/meson-*" ! -path "*/subprojects/*" ! -name ".*" ! -name "*.so" ! -name "*.dylib" ! -name "*.a" ! -name "*.p" ! -name "build.ninja" ! -name "*.json" ! -name "*.dat" -exec cp {} /output/%[8]s/ \; 2>/dev/null || true
# Library/shared objects
find /tmp/builddir -maxdepth 3 -type f \( -name "*.a" -o -name "*.so" -o -name "*.dylib" \) ! -path "*/meson-*" -exec cp {} /output/%[8]s/ \; 2>/dev/null || true
if [ "%[5]s" = "true" ]; then ls -la /output/%[8]s/ 2>/dev/null || echo "  (no artifacts found)"; fi
%[12]s
%[9]s%[10]s%[11]s
`, envExports, setupEcho, strings.Join(setupArgs, " "), mesonQuiet, isVerbose, buildEcho, copyEcho, opts.TargetName, testSection, benchSection, runSection, buildCompleteEcho, projectName)

	fmt.Printf("  %s Running Meson build in Docker container...%s\n", colors.Cyan, colors.Reset)

	dockerArgs := []string{"run", "--rm"}
	if opts.Platform != "" {
		dockerArgs = append(dockerArgs, "--platform", opts.Platform)
	}

	dockerArgs = append(dockerArgs,
		"-v", absProjectRoot+":/workspace:ro",
		"-v", absBuildDir+":/tmp/builddir",
		"-v", absSubprojectsDir+":/workspace/subprojects",
		"-v", absOutputDir+":/output",
		"-w", "/workspace",
		opts.ImageName,
		"bash", "-c", buildScript)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker meson build failed: %w", err)
	}

	return nil
}

var _ build.DockerBuilder = (*Builder)(nil)
