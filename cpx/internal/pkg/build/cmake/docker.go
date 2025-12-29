// Package cmake provides CMake-only build system Docker integration.
package cmake

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build/common"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
)

func (b *Builder) RunDockerBuild(opts build.DockerBuildOptions) error {
	// Create target-specific output directory
	targetOutputDir := filepath.Join(opts.OutputDir, opts.TargetName)
	if err := os.MkdirAll(targetOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create target output directory: %w", err)
	}

	// Detect project type (executable or library)
	isExe, err := detectCMakeProjectType(opts.ProjectRoot)
	if err != nil {
		isExe = true // default to executable
	}

	// Determine build type
	buildType := opts.BuildType
	if buildType == "" {
		buildType = "Release"
	}

	optLevel := opts.Optimization
	if optLevel == "" {
		optLevel = "2"
	}

	// Create a persistent build directory for this target
	hostBuildDir := filepath.Join(opts.ProjectRoot, ".cache", "ci", opts.TargetName)
	if err := os.MkdirAll(hostBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	absBuildDir, err := filepath.Abs(hostBuildDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for build directory: %w", err)
	}

	containerBuildDir := "/tmp/build"

	// Build CMake arguments
	cmakeArgs := []string{
		"-GNinja",
		"-B", containerBuildDir,
		"-S", "/workspace",
		"-DCMAKE_BUILD_TYPE=" + buildType,
		"-DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
	}

	if opts.RunTests {
		cmakeArgs = append(cmakeArgs, "-DBUILD_TESTING=ON", "-DENABLE_TESTING=ON")
	}

	if opts.RunBenchmarks {
		cmakeArgs = append(cmakeArgs, "-DENABLE_BENCHMARKS=ON")
	}

	cmakeArgs = append(cmakeArgs, "-DCMAKE_CXX_FLAGS=-O"+optLevel)
	cmakeArgs = append(cmakeArgs, opts.CMakeArgs...)

	// Build command arguments
	buildArgs := []string{"--build", containerBuildDir, "--config", buildType}
	if opts.Jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", opts.Jobs))
	}
	buildArgs = append(buildArgs, opts.BuildArgs...)

	// Get project name
	projectName := common.GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = filepath.Base(opts.ProjectRoot)
	}

	if opts.RunBenchmarks {
		buildArgs = append(buildArgs, "--target", "all", projectName+"_bench")
	}

	// Determine artifact copying
	var copyCommand string
	if isExe {
		copyCommand = fmt.Sprintf(`find %s -maxdepth 2 -type f -executable ! -name "CMake*" ! -name "*.py" ! -name "*.sh" ! -name "*.sample" ! -name "a.out" ! -name "*.cmake" ! -path "*/CMakeFiles/*" -exec cp {} /output/%s/ \; 2>/dev/null || true
find %s -maxdepth 2 -type f \( -name "lib*.a" -o -name "lib*.so" -o -name "lib*.dylib" \) ! -path "*/CMakeFiles/*" -exec cp {} /output/%s/ \; 2>/dev/null || true`, containerBuildDir, opts.TargetName, containerBuildDir, opts.TargetName)
	} else {
		copyCommand = fmt.Sprintf(`find %s -maxdepth 2 -type f \( -name "lib*.a" -o -name "lib*.so" -o -name "lib*.dylib" \) ! -path "*/CMakeFiles/*" -exec cp {} /output/%s/ \; 2>/dev/null || true`, containerBuildDir, opts.TargetName)
	}

	absOutputDir, err := filepath.Abs(filepath.Join(opts.ProjectRoot, opts.OutputDir))
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output directory: %w", err)
	}

	// Environment exports
	var envExports string
	if len(opts.Env) > 0 {
		envExports = "# User-defined environment variables\n"
		for k, v := range opts.Env {
			envExports += fmt.Sprintf("export %s=\"%s\"\n", k, v)
		}
	}

	testSection := ""
	if opts.RunTests {
		testSection = fmt.Sprintf(`
echo " Running tests..."
cd %s
ctest --output-on-failure
cd - > /dev/null
`, containerBuildDir)
	}

	benchSection := ""
	if opts.RunBenchmarks {
		benchSection = fmt.Sprintf(`
echo " Running benchmarks..."
cd %s
for bench in $(find . -maxdepth 2 -type f -executable -name "*_bench" 2>/dev/null); do
    echo "  Running $bench..."
    $bench
done
cd - > /dev/null
`, containerBuildDir)
	}

	// Execute after build section
	runSection := ""
	if opts.ExecuteAfterBuild {
		runSection = fmt.Sprintf(`
echo " Running executable..."
cd %s
EXEC=""
if [ -f "./%[2]s" ] && [ -x "./%[2]s" ]; then
    EXEC="./%[2]s"
else
    for f in $(find . -maxdepth 2 -type f -perm /111 ! -name "CMake*" ! -name "*.py" ! -name "*.sh" ! -name "*.json" ! -name "*.sample" ! -name "a.out" ! -name "*.cmake" ! -path "*/CMakeFiles/*" ! -name "*_test*" ! -name "*_bench" 2>/dev/null); do
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
`, containerBuildDir, projectName)
	}

	// Determine final steps based on whether we run the executable
	finalSteps := ""
	if opts.ExecuteAfterBuild {
		finalSteps = fmt.Sprintf(`echo " Copying artifacts..."
mkdir -p /output/%s
%s
%s`, opts.TargetName, copyCommand, runSection)
	} else {
		finalSteps = fmt.Sprintf(`echo " Copying artifacts..."
mkdir -p /output/%s
%s
echo " Build complete!"`, opts.TargetName, copyCommand)
	}

	// Handle verbosity for CMake commands
	cmakeQuiet := ""
	if !opts.Verbose {
		cmakeQuiet = " > /dev/null 2>&1"
	}

	configEcho := "echo \"  Configuring CMake (Ninja)...\""
	buildEcho := "echo \" Building...\""
	if !opts.Verbose {
		configEcho = ":"
		buildEcho = ":"
	}

	buildScript := fmt.Sprintf(`#!/bin/bash
set -e
%smkdir -p %s
%s
cmake %s%s
%s
cmake %s%s
%s%s%s
`, envExports, containerBuildDir, configEcho, strings.Join(cmakeArgs, " "), cmakeQuiet, buildEcho, strings.Join(buildArgs, " "), cmakeQuiet, testSection, benchSection, finalSteps)

	// Run Docker container
	fmt.Printf("  %s Running CMake build in Docker container...%s\n", colors.Cyan, colors.Reset)

	dockerArgs := []string{"run", "--rm"}
	if opts.Platform != "" {
		dockerArgs = append(dockerArgs, "--platform", opts.Platform)
	}

	absProjectRoot, err := filepath.Abs(opts.ProjectRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for project root: %w", err)
	}

	dockerArgs = append(dockerArgs,
		"-v", absProjectRoot+":/workspace:ro",
		"-v", absBuildDir+":/tmp/build",
		"-v", absOutputDir+":/output",
		"-w", "/workspace",
		opts.ImageName,
		"bash", "-c", buildScript)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker cmake build failed: %w", err)
	}

	return nil
}

func detectCMakeProjectType(projectRoot string) (bool, error) {
	cmakeListsPath := filepath.Join(projectRoot, "CMakeLists.txt")
	data, err := os.ReadFile(cmakeListsPath)
	if err != nil {
		return false, fmt.Errorf("failed to read CMakeLists.txt: %w", err)
	}

	content := string(data)
	if strings.Contains(content, "add_executable") {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "add_executable(") {
				if !strings.Contains(trimmed, "_tests") && !strings.Contains(trimmed, "_test") {
					return true, nil
				}
			}
		}
		if strings.Contains(content, "add_library") {
			return false, nil
		}
		return true, nil
	}

	if strings.Contains(content, "add_library") {
		return false, nil
	}

	return true, nil
}

var _ build.DockerBuilder = (*Builder)(nil)
