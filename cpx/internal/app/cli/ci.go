package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	"github.com/ozacod/cpx/internal/pkg/build/common"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
	"github.com/ozacod/cpx/pkg/config"
)

type ToolchainBuildOptions struct {
	ToolchainName     string
	Rebuild           bool
	ExecuteAfterBuild bool
	RunTests          bool
	RunBenchmarks     bool
	Verbose           bool
}

func runToolchainBuild(options ToolchainBuildOptions) error {
	ciConfig, err := config.LoadToolchains("cpx-ci.yaml")
	if err != nil {
		return fmt.Errorf("failed to load cpx-ci.yaml: %w\n  Create cpx-ci.yaml file or run 'cpx build' for local builds", err)
	}

	// Get toolchains to run
	toolchains := ciConfig.Toolchains
	if options.ToolchainName != "" {
		found := false
		for _, t := range ciConfig.Toolchains {
			if t.Name == options.ToolchainName {
				toolchains = []config.Toolchain{t}
				found = true
				if !t.IsActive() {
					fmt.Printf("%sWarning: Toolchain '%s' is marked as inactive%s\n", colors.Yellow, options.ToolchainName, colors.Reset)
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("toolchain '%s' not found in cpx-ci.yaml", options.ToolchainName)
		}
	} else {
		var activeToolchains []config.Toolchain
		var skippedCount int
		for _, t := range ciConfig.Toolchains {
			if t.IsActive() {
				activeToolchains = append(activeToolchains, t)
			} else {
				skippedCount++
			}
		}
		if skippedCount > 0 {
			fmt.Printf("%sSkipping %d inactive toolchain(s)%s\n", colors.Yellow, skippedCount, colors.Reset)
		}
		toolchains = activeToolchains
	}

	if len(toolchains) == 0 {
		return fmt.Errorf("no active toolchains defined in cpx-ci.yaml")
	}

	outputDir := ciConfig.GetOutputDir()
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("%s Building %d toolchain(s)...%s\n", colors.Cyan, len(toolchains), colors.Reset)

	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	for i, tc := range toolchains {
		// Resolve runner (contains compiler settings too)
		runner := ciConfig.FindRunner(tc.Runner)
		if runner == nil && tc.Runner != "" {
			return fmt.Errorf("runner '%s' not found for toolchain '%s'", tc.Runner, tc.Name)
		}

		// Determine runner type
		runnerType := "native"
		if runner != nil && runner.Type != "" {
			runnerType = runner.Type
		}

		if options.ExecuteAfterBuild {
			fmt.Printf("\n%s[%d/%d] Building and running: %s (%s)%s\n", colors.Cyan, i+1, len(toolchains), tc.Name, runnerType, colors.Reset)
		} else {
			fmt.Printf("\n%s[%d/%d] Building: %s (%s)%s\n", colors.Cyan, i+1, len(toolchains), tc.Name, runnerType, colors.Reset)
		}

		// Build environment with compiler settings from runner
		env := tc.Env
		if env == nil {
			env = make(map[string]string)
		}
		if runner != nil {
			if runner.CC != "" {
				env["CC"] = runner.CC
			}
			if runner.CXX != "" {
				env["CXX"] = runner.CXX
			}
		}

		// Get CMake toolchain file if specified in runner
		cmakeToolchainFile := ""
		if runner != nil && runner.CMakeToolchainFile != "" {
			cmakeToolchainFile = runner.CMakeToolchainFile
		}

		if runner == nil || runner.IsNative() {
			if err := runNativeBuildNew(tc, runner, projectRoot, outputDir, options.RunTests, options.RunBenchmarks); err != nil {
				return fmt.Errorf("failed to build '%s': %w", tc.Name, err)
			}
		} else if runner.IsDocker() {
			imageName, err := resolveDockerImageNew(runner)
			if err != nil {
				return fmt.Errorf("failed to resolve Docker image for '%s': %w", tc.Name, err)
			}

			var dockerBuilder build.DockerBuilder
			if _, err := os.Stat(filepath.Join(projectRoot, "MODULE.bazel")); err == nil {
				dockerBuilder = bazel.New()
			} else if _, err := os.Stat(filepath.Join(projectRoot, "meson.build")); err == nil {
				dockerBuilder = meson.New()
			} else {
				dockerBuilder = vcpkg.New()
			}

			// Set defaults for optimization and jobs if not specified in toolchain
			optLevel := tc.Optimization
			if optLevel == "" {
				optLevel = "2"
			}
			jobs := tc.Jobs

			opts := build.DockerBuildOptions{
				ImageName:         imageName,
				ProjectRoot:       projectRoot,
				OutputDir:         outputDir,
				BuildType:         tc.BuildType,
				Optimization:      optLevel,
				CMakeArgs:         tc.CMakeOptions,
				BuildArgs:         tc.BuildOptions,
				Jobs:              jobs,
				Env:               env,
				ExecuteAfterBuild: options.ExecuteAfterBuild,
				RunTests:          options.RunTests,
				RunBenchmarks:     options.RunBenchmarks,
				TargetName:        tc.Name,
				Verbose:           options.Verbose,
			}

			// Add toolchain file to CMake args if specified
			if cmakeToolchainFile != "" {
				opts.CMakeArgs = append(opts.CMakeArgs, "-DCMAKE_TOOLCHAIN_FILE="+cmakeToolchainFile)
			}

			if err := dockerBuilder.RunDockerBuild(context.Background(), opts); err != nil {
				return fmt.Errorf("failed to build '%s': %w", tc.Name, err)
			}
		} else if runner.IsSSH() {
			return fmt.Errorf("SSH runner not yet implemented for toolchain '%s'", tc.Name)
		}

		if !options.ExecuteAfterBuild {
			fmt.Printf("%s Build '%s' succeeded%s\n", colors.Green, tc.Name, colors.Reset)
		}
	}

	if !options.ExecuteAfterBuild {
		fmt.Printf("\n%s All builds completed successfully!%s\n", colors.Green, colors.Reset)
		fmt.Printf("   Artifacts are in: %s\n", outputDir)
	}
	return nil
}

func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Check for various project markers
	markers := []string{"CMakeLists.txt", "vcpkg.json", "meson.build", "MODULE.bazel", ".git"}
	dir := cwd
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd, nil
}

// resolveDockerImageNew verifies the Docker image exists locally
func resolveDockerImageNew(runner *config.Runner) (string, error) {
	if runner.Image == "" {
		return "", fmt.Errorf("Docker runner '%s' has no image specified", runner.Name)
	}
	imageName := runner.Image

	// Check if image exists locally
	cmd := exec.Command("docker", "images", "-q", imageName)
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return "", fmt.Errorf("Docker image '%s' not found locally. Use 'docker pull %s' to download it first", imageName, imageName)
	}

	fmt.Printf("  %s Using Docker image: %s%s\n", colors.Green, imageName, colors.Reset)
	return imageName, nil
}

// runNativeBuildNew runs a native CMake build with new config structure
func runNativeBuildNew(tc config.Toolchain, runner *config.Runner, projectRoot, outputDir string, runTests bool, runBenchmarks bool) error {
	projectType := DetectProjectType()
	missing := WarnMissingBuildTools(projectType)
	if len(missing) > 0 {
		fmt.Printf("  %sNote: Native build may fail due to missing tools%s\n", colors.Yellow, colors.Reset)
	}

	targetOutputDir := filepath.Join(outputDir, tc.Name)
	if err := os.MkdirAll(targetOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create target output directory: %w", err)
	}

	hostBuildDir := filepath.Join(projectRoot, ".cache", "ci", tc.Name)
	if err := os.MkdirAll(hostBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	absBuildDir, err := filepath.Abs(hostBuildDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for build directory: %w", err)
	}
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for project root: %w", err)
	}
	absOutputDir, err := filepath.Abs(targetOutputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output directory: %w", err)
	}

	buildType := tc.BuildType
	if buildType == "" {
		buildType = "Release"
	}

	optLevel := tc.Optimization
	if optLevel == "" {
		optLevel = "2"
	}

	cmakeArgs := []string{
		"-GNinja",
		"-B", absBuildDir,
		"-S", absProjectRoot,
		"-DCMAKE_BUILD_TYPE=" + buildType,
		"-DCMAKE_CXX_FLAGS=-O" + optLevel,
	}

	// Add toolchain file if specified in runner
	if runner != nil && runner.CMakeToolchainFile != "" {
		cmakeArgs = append(cmakeArgs, "-DCMAKE_TOOLCHAIN_FILE="+runner.CMakeToolchainFile)
	}

	if runTests {
		cmakeArgs = append(cmakeArgs, "-DBUILD_TESTING=ON", "-DENABLE_TESTING=ON")
	}

	if runBenchmarks {
		cmakeArgs = append(cmakeArgs, "-DENABLE_BENCHMARKS=ON")
	}

	cmakeArgs = append(cmakeArgs, tc.CMakeOptions...)

	// Set environment variables
	env := os.Environ()
	if runner != nil {
		if runner.CC != "" {
			env = append(env, "CC="+runner.CC)
		}
		if runner.CXX != "" {
			env = append(env, "CXX="+runner.CXX)
		}
	}
	for k, v := range tc.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	fmt.Printf("  %s Configuring CMake (Ninja)...%s\n", colors.Yellow, colors.Reset)
	cmd := exec.Command("cmake", cmakeArgs...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmake configure failed: %w", err)
	}

	fmt.Printf("  %s Building...%s\n", colors.Cyan, colors.Reset)
	buildArgs := []string{"--build", absBuildDir, "--config", buildType}
	if tc.Jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", tc.Jobs))
	}
	buildArgs = append(buildArgs, tc.BuildOptions...)

	if runBenchmarks {
		projectName := common.GetProjectNameFromCMakeLists()
		if projectName == "" {
			projectName = filepath.Base(projectRoot)
		}
		buildArgs = append(buildArgs, "--target", "all", projectName+"_bench")
	}

	cmd = exec.Command("cmake", buildArgs...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmake build failed: %w", err)
	}

	// Copy outputs
	fmt.Printf("  %s Copying artifacts...%s\n", colors.Yellow, colors.Reset)

	// Find executable
	entries, err := os.ReadDir(absBuildDir)
	if err != nil {
		return fmt.Errorf("failed to read build directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		// Check if file is executable (unix) or .exe (windows)
		if info.Mode()&0111 != 0 || strings.HasSuffix(entry.Name(), ".exe") {
			src := filepath.Join(absBuildDir, entry.Name())
			dst := filepath.Join(absOutputDir, entry.Name())
			if err := copyFile(src, dst); err != nil {
				fmt.Printf("  %sWarning: failed to copy %s: %v%s\n", colors.Yellow, entry.Name(), err, colors.Reset)
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}
