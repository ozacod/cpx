// Package bazel provides Bazel build system Docker integration.
package bazel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/utils/colors"
)

func (b *BazelBuilder) RunDockerBuild(opts build.DockerBuildOptions) error {
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

	// Create bazel cache directory
	bazelCacheDir := filepath.Join(absProjectRoot, ".cache", "ci", opts.TargetName)
	if err := os.MkdirAll(bazelCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create bazel cache directory: %w", err)
	}

	// Determine build config
	bazelConfig := "release"
	if opts.BuildType == "Debug" || opts.BuildType == "debug" {
		bazelConfig = "debug"
	}

	// Create bazel repository cache directory
	bazelRepoCacheDir := filepath.Join(absProjectRoot, ".cache", "ci", "bazel_repo_cache")
	if err := os.MkdirAll(bazelRepoCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create bazel repo cache directory: %w", err)
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
		testSection = `
echo "  Running tests..."
bazel --output_base="$BAZEL_OUTPUT_BASE" test --config=debug --symlink_prefix=/dev/null --spawn_strategy=local --repository_cache=/bazel-repo-cache --test_output=errors //...
`
	}

	benchSection := ""
	if opts.RunBenchmarks {
		benchSection = `
echo "  Running benchmarks..."
bazel --output_base="$BAZEL_OUTPUT_BASE" run --config=release --symlink_prefix=/dev/null --spawn_strategy=local --repository_cache=/bazel-repo-cache //bench/...
`
	}

	// Handle verbosity
	bazelQuiet := ""
	if !opts.Verbose {
		bazelQuiet = " > /dev/null 2>&1"
	}

	buildEcho := "echo \"  Building with Bazel...\""
	copyEcho := "echo \"  Copying artifacts...\""
	if !opts.Verbose {
		buildEcho = ":"
		copyEcho = ":"
	}

	runSection := ""
	if opts.ExecuteAfterBuild {
		runSection = `
echo "  Running executable..."
cd /output/%s
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:.
EXEC=""
if [ -f "./%[2]s" ] && [ -x "./%[2]s" ]; then
    EXEC="./%[2]s"
else
    for f in $(find . -maxdepth 1 -type f -executable ! -name "*.so" ! -name "*.dylib" ! -name "*.a" 2>/dev/null); do
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
		runSection = fmt.Sprintf(runSection, opts.TargetName, opts.TargetName)
	}

	buildCompleteEcho := "echo \"  Build complete!\""
	if opts.ExecuteAfterBuild {
		buildCompleteEcho = ":"
	}

	buildScript := fmt.Sprintf(`#!/bin/bash
set -e
%[1]s%[2]s
export HOME=/root
BAZEL_OUTPUT_BASE=/bazel-cache
mkdir -p "$BAZEL_OUTPUT_BASE"
bazel --output_base="$BAZEL_OUTPUT_BASE" build --config=%[3]s --symlink_prefix=/dev/null --spawn_strategy=local --repository_cache=/bazel-repo-cache //...%[4]s
%[5]s
mkdir -p /output/%[6]s
find "$BAZEL_OUTPUT_BASE" -path "*/bin/*" -type f -executable \
    ! -name "*.o" ! -name "*.d" ! -name "*.a" ! -name "*.so" ! -name "*.dylib" \
    ! -name "*.runfiles*" ! -name "*.params" ! -name "*.sh" ! -name "*.py" \
    ! -name "*.repo_mapping" ! -name "*.cppmap" ! -name "MANIFEST" \
    ! -name "*.pic.o" ! -name "*.pic.d" \
    -exec cp {} /output/%[6]s/ \; 2>/dev/null || true
find "$BAZEL_OUTPUT_BASE" -path "*/bin/*" -type f \( -name "lib*.a" -o -name "lib*.so" \) \
    ! -name "*.pic.a" \
    -exec cp {} /output/%[6]s/ \; 2>/dev/null || true
%[10]s
%[7]s%[8]s%[9]s
`, envExports, buildEcho, bazelConfig, bazelQuiet, copyEcho, opts.TargetName, testSection, benchSection, runSection, buildCompleteEcho)

	fmt.Printf("  %s Running Bazel build in Docker container...%s\n", colors.Cyan, colors.Reset)

	dockerArgs := []string{"run", "--rm"}
	if opts.Platform != "" {
		dockerArgs = append(dockerArgs, "--platform", opts.Platform)
	}

	dockerArgs = append(dockerArgs,
		"-v", absProjectRoot+":/workspace:ro",
		"-v", absOutputDir+":/output",
		"-v", bazelCacheDir+":/bazel-cache",
		"-v", bazelRepoCacheDir+":/bazel-repo-cache",
		"-w", "/workspace",
		opts.ImageName,
		"bash", "-c", buildScript)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker bazel build failed: %w", err)
	}

	return nil
}

var _ build.DockerBuilder = (*BazelBuilder)(nil)
