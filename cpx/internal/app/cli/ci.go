package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	"github.com/ozacod/cpx/internal/pkg/build/cmake"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
	"github.com/ozacod/cpx/pkg/config"
)

var ciCommandExecuted = false

type ToolchainBuildOptions struct {
	ToolchainName     string
	Rebuild           bool
	ExecuteAfterBuild bool
	RunTests          bool
	RunBenchmarks     bool
}

func runToolchainBuild(options ToolchainBuildOptions) error {
	if ciCommandExecuted {
		fmt.Printf("%s[DEBUG] CI command already executed in this process (PID: %d), skipping second invocation.%s\n", colors.Yellow, os.Getpid(), colors.Reset)
		return nil
	}
	ciCommandExecuted = true

	ciConfig, err := config.LoadToolchains("cpx-ci.yaml")
	if err != nil {
		return fmt.Errorf("failed to load cpx-ci.yaml: %w\n  Create cpx-ci.yaml file or run 'cpx build' for local builds", err)
	}

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

	outputDir := ciConfig.Output
	if outputDir == "" {
		outputDir = filepath.Join(".bin", "ci")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("%s Building for %d toolchain(s)...%s\n", colors.Cyan, len(toolchains), colors.Reset)

	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	cacheBaseDir := filepath.Join(projectRoot, ".cache", "ci")
	if err := os.MkdirAll(cacheBaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	for _, tc := range toolchains {
		if tc.Runner == "docker" && tc.Docker != nil {
			tcCacheDir := filepath.Join(cacheBaseDir, tc.Name, ".vcpkg_cache")
			if err := os.MkdirAll(tcCacheDir, 0755); err != nil {
				return fmt.Errorf("failed to create toolchain cache directory: %w", err)
			}
		}
	}

	for i, tc := range toolchains {
		if options.ExecuteAfterBuild {
			fmt.Printf("\n%s[%d/%d] Building and running toolchain: %s (%s)%s\n", colors.Cyan, i+1, len(toolchains), tc.Name, tc.Runner, colors.Reset)
		} else {
			fmt.Printf("\n%s[%d/%d] Building toolchain: %s (%s)%s\n", colors.Cyan, i+1, len(toolchains), tc.Name, tc.Runner, colors.Reset)
		}

		if tc.Runner == "native" {
			if err := runNativeBuild(tc, projectRoot, outputDir, ciConfig.Build, options.RunTests, options.RunBenchmarks); err != nil {
				return fmt.Errorf("failed to build toolchain %s: %w", tc.Name, err)
			}
		} else {
			imageName, err := resolveDockerImage(tc, projectRoot, options.Rebuild)
			if err != nil {
				return fmt.Errorf("failed to resolve Docker image for %s: %w", tc.Name, err)
			}

			var dockerBuilder build.DockerBuilder
			if _, err := os.Stat(filepath.Join(projectRoot, "MODULE.bazel")); err == nil {
				dockerBuilder = bazel.New()
			} else if _, err := os.Stat(filepath.Join(projectRoot, "meson.build")); err == nil {
				dockerBuilder = meson.New()
			} else {
				dockerBuilder = vcpkg.New()
			}

			platform := ""
			if tc.Docker != nil {
				platform = tc.Docker.Platform
			}

			opts := build.DockerBuildOptions{
				ImageName:         imageName,
				ProjectRoot:       projectRoot,
				OutputDir:         outputDir,
				BuildType:         tc.BuildType,
				Optimization:      ciConfig.Build.Optimization,
				CMakeArgs:         ciConfig.Build.CMakeArgs,
				BuildArgs:         ciConfig.Build.BuildArgs,
				MesonArgs:         ciConfig.Build.MesonArgs,
				Jobs:              ciConfig.Build.Jobs,
				Env:               tc.Env,
				ExecuteAfterBuild: options.ExecuteAfterBuild,
				RunTests:          options.RunTests,
				RunBenchmarks:     options.RunBenchmarks,
				Platform:          platform,
				TargetName:        tc.Name,
			}

			if len(tc.CMakeOptions) > 0 {
				opts.CMakeArgs = tc.CMakeOptions
			}
			if len(tc.BuildOptions) > 0 {
				opts.BuildArgs = tc.BuildOptions
			}
			if tc.BuildType != "" {
				opts.BuildType = tc.BuildType
			}

			if err := dockerBuilder.RunDockerBuild(context.Background(), opts); err != nil {
				return fmt.Errorf("failed to build toolchain %s: %w", tc.Name, err)
			}
		}

		if options.ExecuteAfterBuild {
			fmt.Printf("%s Toolchain %s completed%s\n", colors.Green, tc.Name, colors.Reset)
		} else {
			fmt.Printf("%s Toolchain %s built successfully%s\n", colors.Green, tc.Name, colors.Reset)
		}
	}

	if !options.ExecuteAfterBuild {
		fmt.Printf("\n%s All toolchains built successfully!%s\n", colors.Green, colors.Reset)
		fmt.Printf("   Artifacts are in: %s\n", outputDir)
	}
	return nil
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for project markers
	for {
		// Check for cpx-ci.yaml or CMakeLists.txt or MODULE.bazel (project markers)
		if _, err := os.Stat(filepath.Join(dir, "cpx-ci.yaml")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "CMakeLists.txt")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "MODULE.bazel")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "meson.build")); err == nil {
			return dir, nil
		}

		// Check if we've reached the root
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, return current directory
			return os.Getwd()
		}
		dir = parent
	}
}

// hashDockerBuildConfig computes a hash of Dockerfile content + build args
// Returns first 12 characters of the SHA256 hash
func hashDockerBuildConfig(dockerfilePath string, args map[string]string) (string, error) {
	// Read Dockerfile content
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read Dockerfile: %w", err)
	}

	// Create hash input: dockerfile content + sorted args
	h := sha256.New()
	h.Write(content)

	// Sort args keys for deterministic hashing
	if len(args) > 0 {
		keys := make([]string, 0, len(args))
		for k := range args {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			h.Write([]byte(k))
			h.Write([]byte("="))
			h.Write([]byte(args[k]))
			h.Write([]byte("\n"))
		}
	}

	// Return first 12 chars of hex hash
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

// resolveDockerImage resolves the Docker image based on target configuration
// Returns the image name/tag to use for running the container
func resolveDockerImage(target config.Toolchain, projectRoot string, rebuild bool) (string, error) {
	if target.Docker == nil {
		return "", fmt.Errorf("docker configuration is required for docker runner")
	}

	switch target.Docker.Mode {
	case "pull":
		return handlePullMode(target, rebuild)
	case "local":
		return handleLocalMode(target)
	case "build":
		return handleBuildMode(target, projectRoot, rebuild)
	default:
		return "", fmt.Errorf("unknown docker mode: %s", target.Docker.Mode)
	}
}

// handlePullMode handles the "pull" Docker mode
func handlePullMode(target config.Toolchain, rebuild bool) (string, error) {
	imageName := target.Docker.Image
	pullPolicy := target.Docker.PullPolicy

	// Check if image exists locally
	imageExists := false
	cmd := exec.Command("docker", "images", "-q", imageName)
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		imageExists = true
	}

	// Determine if we should pull
	shouldPull := false
	switch pullPolicy {
	case "always":
		shouldPull = true
	case "never":
		if !imageExists {
			return "", fmt.Errorf("image %s not found locally and pullPolicy is 'never'", imageName)
		}
		shouldPull = false
	case "ifNotPresent", "":
		shouldPull = !imageExists
	default:
		return "", fmt.Errorf("unknown pullPolicy: %s", pullPolicy)
	}

	// Force pull if rebuild is requested
	if rebuild {
		shouldPull = true
	}

	if shouldPull {
		fmt.Printf("  %s Pulling Docker image: %s...%s\n", colors.Cyan, imageName, colors.Reset)
		pullArgs := []string{"pull"}
		if target.Docker.Platform != "" {
			pullArgs = append(pullArgs, "--platform", target.Docker.Platform)
		}
		pullArgs = append(pullArgs, imageName)

		cmd := exec.Command("docker", pullArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("docker pull failed: %w", err)
		}
		fmt.Printf("  %s Docker image %s pulled successfully%s\n", colors.Green, imageName, colors.Reset)
	} else {
		fmt.Printf("  %s Docker image %s already exists%s\n", colors.Green, imageName, colors.Reset)
	}

	return imageName, nil
}

// handleLocalMode handles the "local" Docker mode
func handleLocalMode(target config.Toolchain) (string, error) {
	imageName := target.Docker.Image

	// Verify image exists locally
	cmd := exec.Command("docker", "images", "-q", imageName)
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return "", fmt.Errorf("local image %s not found. Use 'docker pull' or 'docker build' to create it", imageName)
	}

	fmt.Printf("  %s Using local Docker image: %s%s\n", colors.Green, imageName, colors.Reset)
	return imageName, nil
}

// handleBuildMode handles the "build" Docker mode with content-based hashing
func handleBuildMode(target config.Toolchain, projectRoot string, rebuild bool) (string, error) {
	if target.Docker.Build == nil {
		return "", fmt.Errorf("build configuration is required for mode: build")
	}

	// Resolve Dockerfile path
	dockerfilePath := target.Docker.Build.Dockerfile
	if !filepath.IsAbs(dockerfilePath) {
		dockerfilePath = filepath.Join(projectRoot, dockerfilePath)
	}

	// Verify Dockerfile exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("dockerfile not found: %s", dockerfilePath)
	}

	// Compute hash from Dockerfile + build args
	hash, err := hashDockerBuildConfig(dockerfilePath, target.Docker.Build.Args)
	if err != nil {
		return "", err
	}

	// Generate tag: cpx/<target_name>:<hash>
	imageName := fmt.Sprintf("cpx/%s:%s", target.Name, hash)

	// Check if image with exact tag exists
	if !rebuild {
		cmd := exec.Command("docker", "images", "-q", imageName)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			fmt.Printf("  %s Docker image %s already exists (hash match)%s\n", colors.Green, imageName, colors.Reset)
			return imageName, nil
		}
	}

	// Build the image
	fmt.Printf("  %s Building Docker image: %s...%s\n", colors.Cyan, imageName, colors.Reset)

	// Resolve build context
	buildContext := target.Docker.Build.Context
	if buildContext == "" {
		buildContext = "."
	}
	if !filepath.IsAbs(buildContext) {
		buildContext = filepath.Join(projectRoot, buildContext)
	}

	// Build Docker image
	buildArgs := []string{"buildx", "build", "-f", dockerfilePath, "-t", imageName}
	if target.Docker.Platform != "" {
		buildArgs = append(buildArgs, "--platform", target.Docker.Platform)
	}
	// Add build args
	for k, v := range target.Docker.Build.Args {
		buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	buildArgs = append(buildArgs, "--load") // Load into local Docker daemon
	buildArgs = append(buildArgs, buildContext)

	cmd := exec.Command("docker", buildArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// If buildx fails, fall back to regular docker build
	if err := cmd.Run(); err != nil {
		fmt.Printf("  %s docker buildx failed, trying regular docker build...%s\n", colors.Yellow, colors.Reset)
		buildArgs = []string{"build", "-f", dockerfilePath, "-t", imageName}
		if target.Docker.Platform != "" {
			buildArgs = append(buildArgs, "--platform", target.Docker.Platform)
		}
		for k, v := range target.Docker.Build.Args {
			buildArgs = append(buildArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}
		buildArgs = append(buildArgs, buildContext)

		cmd = exec.Command("docker", buildArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("docker build failed: %w", err)
		}
	}

	fmt.Printf("  %s Docker image %s built successfully%s\n", colors.Green, imageName, colors.Reset)
	return imageName, nil
}

// runNativeBuild runs a native CMake build on the host system
func runNativeBuild(target config.Toolchain, projectRoot, outputDir string, buildConfig config.ToolchainBuild, runTests bool, runBenchmarks bool) error {
	// Detect project type and check for missing build tools
	projectType := DetectProjectType()
	missing := WarnMissingBuildTools(projectType)
	if len(missing) > 0 {
		fmt.Printf("  %sNote: Native build may fail due to missing tools%s\n", colors.Yellow, colors.Reset)
	}

	// Create target-specific output directory
	targetOutputDir := filepath.Join(outputDir, target.Name)
	if err := os.MkdirAll(targetOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create target output directory: %w", err)
	}

	// Create persistent build directory for caching
	hostBuildDir := filepath.Join(projectRoot, ".cache", "ci", target.Name)
	if err := os.MkdirAll(hostBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Get absolute paths
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

	// Determine build type (per-target overrides global)
	buildType := target.BuildType
	if buildType == "" {
		buildType = buildConfig.Type
	}
	if buildType == "" {
		buildType = "Release"
	}
	optLevel := buildConfig.Optimization
	if optLevel == "" {
		optLevel = "2"
	}

	// Determine CMake and build options (per-target overrides global)
	cmakeOptions := target.CMakeOptions
	if len(cmakeOptions) == 0 {
		cmakeOptions = buildConfig.CMakeArgs
	}
	buildOptions := target.BuildOptions
	if len(buildOptions) == 0 {
		buildOptions = buildConfig.BuildArgs
	}

	// Build CMake arguments
	cmakeArgs := []string{
		"-GNinja",
		"-B", absBuildDir,
		"-S", absProjectRoot,
		"-DCMAKE_BUILD_TYPE=" + buildType,
		"-DCMAKE_CXX_FLAGS=-O" + optLevel,
	}

	if runTests {
		cmakeArgs = append(cmakeArgs, "-DBUILD_TESTING=ON", "-DENABLE_TESTING=ON")
	}

	if runBenchmarks {
		cmakeArgs = append(cmakeArgs, "-DENABLE_BENCHMARKS=ON")
	}

	// Add custom CMake args (per-target or global)
	cmakeArgs = append(cmakeArgs, cmakeOptions...)

	// Set environment variables from target config
	env := os.Environ()
	for k, v := range target.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Always configure to ensure flags are updated
	fmt.Printf("  %s Configuring CMake (Ninja)...%s\n", colors.Yellow, colors.Reset)
	cmd := exec.Command("cmake", cmakeArgs...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmake configure failed: %w", err)
	}

	// Build
	fmt.Printf("  %s Building...%s\n", colors.Cyan, colors.Reset)
	buildArgs := []string{"--build", absBuildDir, "--config", buildType}
	if buildConfig.Jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", buildConfig.Jobs))
	}
	buildArgs = append(buildArgs, buildOptions...)

	if runBenchmarks {
		// Try to get exact project name from CMakeLists.txt
		projectName := cmake.GetProjectNameFromCMakeLists()
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

	// Copy artifacts
	fmt.Printf("  %s Copying artifacts...%s\n", colors.Cyan, colors.Reset)

	// Find and copy executables
	entries, err := os.ReadDir(absBuildDir)
	if err != nil {
		return fmt.Errorf("failed to read build directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip non-artifacts
		if strings.HasSuffix(name, ".ninja") || strings.HasSuffix(name, ".cmake") ||
			strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".json") ||
			strings.HasPrefix(name, "CMake") {
			continue
		}

		srcPath := filepath.Join(absBuildDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check if file is executable or a library
		isExec := info.Mode()&0111 != 0
		isLib := strings.HasPrefix(name, "lib") && (strings.HasSuffix(name, ".a") ||
			strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".dylib"))

		if isExec || isLib {
			dstPath := filepath.Join(absOutputDir, name)
			input, err := os.ReadFile(srcPath)
			if err != nil {
				continue
			}
			if err := os.WriteFile(dstPath, input, info.Mode()); err != nil {
				continue
			}
			fmt.Printf("    Copied: %s\n", name)
		}
	}

	fmt.Printf("  %s Build complete!%s\n", colors.Green, colors.Reset)

	if runTests {
		fmt.Printf("  %s Running tests...%s\n", colors.Cyan, colors.Reset)
		testCmd := exec.Command("ctest", "--test-dir", absBuildDir, "--output-on-failure")
		testCmd.Stdout = os.Stdout
		testCmd.Stderr = os.Stderr
		if err := testCmd.Run(); err != nil {
			return fmt.Errorf("tests failed: %w", err)
		}
	}

	if runBenchmarks {
		fmt.Printf("  %s Running benchmarks...%s\n", colors.Cyan, colors.Reset)
		// Find and run benchmark executables (ending with _bench)
		entries, err := os.ReadDir(absBuildDir)
		if err == nil {
			foundBench := false
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), "_bench") {
					info, err := entry.Info()
					if err == nil && info.Mode()&0111 != 0 {
						foundBench = true
						benchPath := filepath.Join(absBuildDir, entry.Name())
						fmt.Printf("    Running %s...\n", entry.Name())
						benchCmd := exec.Command(benchPath)
						benchCmd.Stdout = os.Stdout
						benchCmd.Stderr = os.Stderr
						if err := benchCmd.Run(); err != nil {
							fmt.Printf("    %sBenchmark %s failed: %v%s\n", colors.Yellow, entry.Name(), err, colors.Reset)
						}
					}
				}
			}
			if !foundBench {
				fmt.Printf("    No benchmarks found (looking for *_bench executables)\n")
			}
		}
	}

	return nil

}
