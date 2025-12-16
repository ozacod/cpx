package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ozacod/cpx/pkg/config"
)

var ciCommandExecuted = false

func runToolchainBuild(toolchainName string, rebuild bool, executeAfterBuild bool) error {
	if ciCommandExecuted {
		fmt.Printf("%s[DEBUG] CI command already executed in this process (PID: %d), skipping second invocation.%s\n", Yellow, os.Getpid(), Reset)
		return nil
	}
	ciCommandExecuted = true

	// Load cpx-ci.yaml configuration
	ciConfig, err := config.LoadToolchains("cpx-ci.yaml")
	if err != nil {
		return fmt.Errorf("failed to load cpx-ci.yaml: %w\n  Create cpx-ci.yaml file or run 'cpx build' for local builds", err)
	}

	// Filter toolchains if specific toolchain requested
	toolchains := ciConfig.Toolchains
	if toolchainName != "" {
		found := false
		for _, t := range ciConfig.Toolchains {
			if t.Name == toolchainName {
				toolchains = []config.Toolchain{t}
				found = true
				// Warn if explicitly targeting an inactive toolchain
				if !t.IsActive() {
					fmt.Printf("%sWarning: Toolchain '%s' is marked as inactive%s\n", Yellow, toolchainName, Reset)
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("toolchain '%s' not found in cpx-ci.yaml", toolchainName)
		}
	} else {
		// Filter out inactive toolchains when building all
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
			fmt.Printf("%sSkipping %d inactive toolchain(s)%s\n", Yellow, skippedCount, Reset)
		}
		toolchains = activeToolchains
	}

	if len(toolchains) == 0 {
		return fmt.Errorf("no active toolchains defined in cpx-ci.yaml")
	}

	// Create output directory
	outputDir := ciConfig.Output
	if outputDir == "" {
		outputDir = filepath.Join(".bin", "ci")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("%s Building for %d toolchain(s)...%s\n", Cyan, len(toolchains), Reset)

	// Get project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	// Pre-create cache directories for all toolchains
	cacheBaseDir := filepath.Join(projectRoot, ".cache", "ci")
	if err := os.MkdirAll(cacheBaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	for _, tc := range toolchains {
		if tc.Runner == "docker" && tc.Docker != nil {
			// Docker toolchains need vcpkg cache
			tcCacheDir := filepath.Join(cacheBaseDir, tc.Name, ".vcpkg_cache")
			if err := os.MkdirAll(tcCacheDir, 0755); err != nil {
				return fmt.Errorf("failed to create toolchain cache directory: %w", err)
			}
		}
	}

	// Build and run for each toolchain
	for i, tc := range toolchains {
		if executeAfterBuild {
			fmt.Printf("\n%s[%d/%d] Building and running toolchain: %s (%s)%s\n", Cyan, i+1, len(toolchains), tc.Name, tc.Runner, Reset)
		} else {
			fmt.Printf("\n%s[%d/%d] Building toolchain: %s (%s)%s\n", Cyan, i+1, len(toolchains), tc.Name, tc.Runner, Reset)
		}

		// Dispatch based on runner type
		if tc.Runner == "native" {
			// Native build
			if err := runNativeBuild(tc, projectRoot, outputDir, ciConfig.Build); err != nil {
				return fmt.Errorf("failed to build toolchain %s: %w", tc.Name, err)
			}
		} else {
			// Docker build (default)
			// Resolve Docker image based on mode
			imageName, err := resolveDockerImage(tc, projectRoot, rebuild)
			if err != nil {
				return fmt.Errorf("failed to resolve Docker image for %s: %w", tc.Name, err)
			}

			// Run build in Docker container
			if err := runDockerBuildWithImage(tc, imageName, projectRoot, outputDir, ciConfig.Build, executeAfterBuild); err != nil {
				return fmt.Errorf("failed to build toolchain %s: %w", tc.Name, err)
			}
		}

		if executeAfterBuild {
			fmt.Printf("%s Toolchain %s completed%s\n", Green, tc.Name, Reset)
		} else {
			fmt.Printf("%s Toolchain %s built successfully%s\n", Green, tc.Name, Reset)
		}
	}

	if !executeAfterBuild {
		fmt.Printf("\n%s All toolchains built successfully!%s\n", Green, Reset)
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
		fmt.Printf("  %s Pulling Docker image: %s...%s\n", Cyan, imageName, Reset)
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
		fmt.Printf("  %s Docker image %s pulled successfully%s\n", Green, imageName, Reset)
	} else {
		fmt.Printf("  %s Docker image %s already exists%s\n", Green, imageName, Reset)
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

	fmt.Printf("  %s Using local Docker image: %s%s\n", Green, imageName, Reset)
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
			fmt.Printf("  %s Docker image %s already exists (hash match)%s\n", Green, imageName, Reset)
			return imageName, nil
		}
	}

	// Build the image
	fmt.Printf("  %s Building Docker image: %s...%s\n", Cyan, imageName, Reset)

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
		fmt.Printf("  %s docker buildx failed, trying regular docker build...%s\n", Yellow, Reset)
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

	fmt.Printf("  %s Docker image %s built successfully%s\n", Green, imageName, Reset)
	return imageName, nil
}

// detectProjectType detects if the project is an executable or library by checking CMakeLists.txt
func detectProjectType(projectRoot string) (bool, error) {
	cmakeListsPath := filepath.Join(projectRoot, "CMakeLists.txt")
	data, err := os.ReadFile(cmakeListsPath)
	if err != nil {
		return false, fmt.Errorf("failed to read CMakeLists.txt: %w", err)
	}

	content := string(data)
	// Check for add_executable (executable project)
	if strings.Contains(content, "add_executable") {
		// Check if it's the main project executable (not test executable)
		// Look for add_executable that's not a test
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "add_executable(") {
				// Check if it's a test executable
				if !strings.Contains(trimmed, "_tests") && !strings.Contains(trimmed, "_test") {
					return true, nil // It's an executable project
				}
			}
		}
		// If we found add_executable but only test executables, check for add_library
		if strings.Contains(content, "add_library") {
			return false, nil // It's a library project
		}
		return true, nil // Default to executable if add_executable exists
	}

	// Check for add_library (library project)
	if strings.Contains(content, "add_library") {
		return false, nil // It's a library project
	}

	// Default: assume executable if we can't determine
	return true, nil
}

// runDockerBuildWithImage runs a Docker build with the specified image name
func runDockerBuildWithImage(target config.Toolchain, imageName, projectRoot, outputDir string, buildConfig config.ToolchainBuild, executeAfterBuild bool) error {
	// Create target-specific output directory
	targetOutputDir := filepath.Join(outputDir, target.Name)
	if err := os.MkdirAll(targetOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create target output directory: %w", err)
	}

	// Check if this is a Bazel project
	isBazel := false
	if _, err := os.Stat(filepath.Join(projectRoot, "MODULE.bazel")); err == nil {
		isBazel = true
	}

	if isBazel {
		return runDockerBazelBuildWithImage(target, imageName, projectRoot, outputDir, buildConfig)
	}

	// Check if this is a Meson project
	if _, err := os.Stat(filepath.Join(projectRoot, "meson.build")); err == nil {
		return runDockerMesonBuildWithImage(target, imageName, projectRoot, outputDir, buildConfig)
	}

	// Detect project type (executable or library) for CMake projects
	isExe, err := detectProjectType(projectRoot)
	if err != nil {
		// If we can't detect, default to executable
		isExe = true
	}

	// vcpkg is installed in the Docker images at /opt/vcpkg
	// No need to mount from host - images are self-contained

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

	// Merge environment variables (target.Env is used in Docker container)
	envVars := target.Env

	// Create a persistent build directory for this target on the host
	// This allows CMake to cache build artifacts (.o files, dependencies, etc.)
	// Location: .cache/ci/<target-name> in the project root
	hostBuildDir := filepath.Join(projectRoot, ".cache", "ci", target.Name)
	if err := os.MkdirAll(hostBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Get absolute path for build directory (Docker requires absolute paths)
	absBuildDir, err := filepath.Abs(hostBuildDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for build directory: %w", err)
	}

	// Use /tmp/build instead of /workspace/build to avoid read-only mount issues
	containerBuildDir := "/tmp/build"

	// Build CMake arguments
	cmakeArgs := []string{
		"-GNinja", // Use Ninja for faster, correct incremental builds
		"-B", containerBuildDir,
		"-S", "/workspace",
		"-DCMAKE_BUILD_TYPE=" + buildType,
		"-DCMAKE_TOOLCHAIN_FILE=/opt/vcpkg/scripts/buildsystems/vcpkg.cmake",
	}
	// Note: VCPKG_INSTALLED_DIR is set via environment variable in the build script
	// This is the recommended way to configure vcpkg cache location

	// Add optimization flags
	cmakeArgs = append(cmakeArgs, "-DCMAKE_CXX_FLAGS=-O"+optLevel)

	// Disable registry updates via CMake variable
	// This is more reliable than environment variables
	cmakeArgs = append(cmakeArgs, "-DVCPKG_DISABLE_REGISTRY_UPDATE=ON")

	// Add custom CMake args (per-target or global)
	cmakeArgs = append(cmakeArgs, cmakeOptions...)

	// Build command arguments
	buildArgs := []string{"--build", containerBuildDir, "--config", buildType}
	if buildConfig.Jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", buildConfig.Jobs))
	}
	buildArgs = append(buildArgs, buildOptions...)

	// Determine artifact copying based on project type
	var copyCommand string
	projectName := filepath.Base(projectRoot)

	if isExe {
		copyCommand = fmt.Sprintf(`# Copy all executables (main, test, bench) and libraries
PROJECT_NAME="%s"
# Copy all executables from build directory (exclude CMake internals)
find %s -maxdepth 2 -type f -executable \
    ! -name "CMake*" ! -name "*.py" ! -name "*.sh" ! -name "*.sample" ! -name "a.out" \
    ! -name "*.cmake" ! -path "*/CMakeFiles/*" \
    -exec cp {} /output/%s/ \; 2>/dev/null || true
# Also copy libraries (static and shared)
find %s -maxdepth 2 -type f \( -name "lib*.a" -o -name "lib*.so" -o -name "lib*.dylib" \) \
    ! -path "*/CMakeFiles/*" \
    -exec cp {} /output/%s/ \; 2>/dev/null || true
# Copy test results if they exist
if [ -f %s/Testing/TAG ]; then
    mkdir -p /output/%s/test_results
    cp -r %s/Testing/* /output/%s/test_results/ 2>/dev/null || true
fi`, projectName, containerBuildDir, target.Name, containerBuildDir, target.Name, containerBuildDir, target.Name, containerBuildDir, target.Name)
	} else {
		copyCommand = fmt.Sprintf(`# Copy all libraries (static and shared)
find %s -maxdepth 2 -type f \( -name "lib*.a" -o -name "lib*.so" -o -name "lib*.dylib" \) \
    ! -path "*/CMakeFiles/*" \
    -exec cp {} /output/%s/ \; 2>/dev/null || true`, containerBuildDir, target.Name)
	}

	// Create persistent vcpkg cache directories under the build directory
	// Mount from host build directory to /tmp/.vcpkg_cache/ in container
	// Use /tmp instead of /workspace to avoid read-only mount issues
	vcpkgCacheDir := filepath.Join(absBuildDir, ".vcpkg_cache")
	vcpkgInstalledDir := filepath.Join(vcpkgCacheDir, "installed")
	vcpkgDownloadsDir := filepath.Join(vcpkgCacheDir, "downloads")
	vcpkgBuildtreesDir := filepath.Join(vcpkgCacheDir, "buildtrees")
	vcpkgBinaryDir := filepath.Join(vcpkgCacheDir, "binary")

	// Create all vcpkg cache directories (must exist before Docker mount)
	if err := os.MkdirAll(vcpkgInstalledDir, 0755); err != nil {
		return fmt.Errorf("failed to create vcpkg installed directory: %w", err)
	}
	if err := os.MkdirAll(vcpkgDownloadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create vcpkg downloads directory: %w", err)
	}
	if err := os.MkdirAll(vcpkgBuildtreesDir, 0755); err != nil {
		return fmt.Errorf("failed to create vcpkg buildtrees directory: %w", err)
	}
	if err := os.MkdirAll(vcpkgBinaryDir, 0755); err != nil {
		return fmt.Errorf("failed to create vcpkg binary cache directory: %w", err)
	}

	// Get absolute paths (Docker requires absolute paths)
	absOutputDir, err := filepath.Abs(filepath.Join(projectRoot, outputDir))
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output directory: %w", err)
	}
	absVcpkgCacheDir, err := filepath.Abs(vcpkgCacheDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for vcpkg cache directory: %w", err)
	}

	// Create build script
	// Use VCPKG_INSTALLED_DIR to persist packages between builds
	// This significantly speeds up subsequent builds by reusing installed packages
	// Use /tmp/.vcpkg_cache instead of /workspace/.vcpkg_cache to avoid read-only mount issues
	vcpkgInstalledPath := "/tmp/.vcpkg_cache/installed"
	vcpkgDownloadsPath := "/tmp/.vcpkg_cache/downloads"
	vcpkgBuildtreesPath := "/tmp/.vcpkg_cache/buildtrees"
	binaryCachePath := "/tmp/.vcpkg_cache/binary"

	// Generate environment variable exports for the build script
	var envExports string
	if len(envVars) > 0 {
		envExports = "# User-defined environment variables\n"
		for k, v := range envVars {
			envExports += fmt.Sprintf("export %s=\"%s\"\n", k, v)
		}
	}

	// Bash build script for Linux/macOS
	buildScript := fmt.Sprintf(`#!/bin/bash
set -e
%sexport VCPKG_ROOT=/opt/vcpkg
export PATH="${VCPKG_ROOT}:${PATH}"
# Set vcpkg to use manifest mode
export VCPKG_FEATURE_FLAGS=manifests
export X_VCPKG_REGISTRIES_CACHE=/tmp/.vcpkg_cache/registries
# Disable registry update check to speed up builds
export VCPKG_DISABLE_REGISTRY_UPDATE=1
# Preserve environment variables in vcpkg's clean build environment
export VCPKG_KEEP_ENV_VARS="VCPKG_DISABLE_REGISTRY_UPDATE;VCPKG_FEATURE_FLAGS;VCPKG_INSTALLED_DIR;VCPKG_DOWNLOADS;VCPKG_BUILDTREES_ROOT;VCPKG_BINARY_SOURCES"
# Set vcpkg cache directories - these persist between builds
export VCPKG_INSTALLED_DIR=%s
export VCPKG_DOWNLOADS=%s
export VCPKG_BUILDTREES_ROOT=%s
# Configure binary caching to reuse built packages
export VCPKG_BINARY_SOURCES="files,%s,readwrite"
# Disable metrics to speed up builds
export VCPKG_DISABLE_METRICS=1
# Ensure directories exist
mkdir -p /tmp/.vcpkg_cache
mkdir -p "$VCPKG_INSTALLED_DIR" "$VCPKG_DOWNLOADS" "$VCPKG_BUILDTREES_ROOT" "%s" "$X_VCPKG_REGISTRIES_CACHE"
# Ensure build directory exists (mounted from host)
mkdir -p %s

# Check if already configured (incremental build)
if [ -f "%s/build.ninja" ]; then
    echo "  Build directory already configured, skipping setup."
else
    echo "  Configuring CMake (Ninja)..."
    cmake %s
fi

echo " Building..."
# Use cmake --build which will re-configure if Build system files changed
cmake %s

echo " Copying artifacts..."
mkdir -p /output/%s
%s
echo " Build complete!"
%s
`, envExports, vcpkgInstalledPath, vcpkgDownloadsPath, vcpkgBuildtreesPath, binaryCachePath, binaryCachePath, containerBuildDir, containerBuildDir, strings.Join(cmakeArgs, " "), strings.Join(buildArgs, " "), target.Name, copyCommand, func() string {
		if executeAfterBuild {
			projectName := filepath.Base(projectRoot)
			return fmt.Sprintf(`
echo ""
echo " Running %s..."
# Try to find the main executable - check common locations
EXEC_PATH=""
# First, check build directory root (prioritize build dir over output)
if [ -x "%s/%s" ]; then
    EXEC_PATH="%s/%s"
# Check if there's an executable with the project name in the output directory
elif [ -x "/output/%s/%s" ]; then
    EXEC_PATH="/output/%s/%s"
else
    # Search for any ELF executable (excluding tests, benchmarks, and libraries)
    for f in $(find %s -maxdepth 3 -type f -executable ! -name "*_test*" ! -name "*_bench*" ! -name "*.a" ! -name "*.so" ! -name "a.out" ! -path "*/CMakeFiles/*" 2>/dev/null | head -5); do
        if file "$f" 2>/dev/null | grep -qE "ELF.*(executable|pie)"; then
            EXEC_PATH="$f"
            break
        fi
    done
fi
if [ -n "$EXEC_PATH" ] && [ -x "$EXEC_PATH" ]; then
    echo " Executing: $EXEC_PATH"
    echo "----------------------------------------"
    "$EXEC_PATH"
    EXIT_CODE=$?
    echo "----------------------------------------"
    echo " Process exited with code: $EXIT_CODE"
else
    echo " No executable found to run"
    echo " Searched for: %s in %s and /output/%s"
fi
`, projectName, containerBuildDir, projectName, containerBuildDir, projectName, target.Name, projectName, target.Name, projectName, containerBuildDir, projectName, containerBuildDir, target.Name)
		}
		return ""
	}())

	// Run Docker container
	fmt.Printf("  %s Running build in Docker container...%s\n", Cyan, Reset)

	// Mount only necessary directories:
	// - Source code (read-only to avoid modifying host files)
	// - Build directory (for caching CMake build artifacts) - mount to a subdirectory that can be created
	// - Output directory (for artifacts)
	// - vcpkg cache directory (from build/.vcpkg_cache to /tmp/.vcpkg_cache)
	dockerArgs := []string{"run", "--rm"}
	// Add platform flag if specified (prevents warning on cross-platform runs)
	if target.Docker != nil && target.Docker.Platform != "" {
		dockerArgs = append(dockerArgs, "--platform", target.Docker.Platform)
	}
	// Mount paths for Linux/macOS containers
	// Build directory is mounted to /tmp/build to avoid read-only /workspace mount issues
	// vcpkg cache is mounted to /tmp/.vcpkg_cache for the same reason
	workspacePath := "/workspace"
	buildPath := "/tmp/build"
	outputPath := "/output"
	cachePath := "/tmp/.vcpkg_cache"
	command := "bash"

	// Get absolute paths for all mounts (Docker requires absolute paths)
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for project root: %w", err)
	}

	// Mounts
	dockerArgs = append(dockerArgs,
		"-v", absProjectRoot+":"+workspacePath+":ro", // Mount source as read-only
		"-v", absBuildDir+":"+buildPath, // Mount build directory for caching build artifacts
		"-v", absOutputDir+":"+outputPath, // Mount output directory for artifacts
		"-v", absVcpkgCacheDir+":"+cachePath, // Mount vcpkg cache
		"-w", workspacePath,
		imageName,
		command, "-c", buildScript)

	cmd := exec.Command("docker", dockerArgs...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}

	return nil
}

// runDockerBazelBuildWithImage runs a Bazel build inside Docker with specified image
func runDockerBazelBuildWithImage(target config.Toolchain, imageName, projectRoot, outputDir string, buildConfig config.ToolchainBuild) error {
	// Get absolute paths
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for project root: %w", err)
	}

	absOutputDir, err := filepath.Abs(filepath.Join(projectRoot, outputDir))
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output directory: %w", err)
	}

	// Create bazel cache directory inside project's .cache directory
	// This keeps the cache with the project and simplifies the mount structure
	bazelCacheDir := filepath.Join(absProjectRoot, ".cache", "ci", target.Name)
	if err := os.MkdirAll(bazelCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create bazel cache directory: %w", err)
	}

	// Determine build config (per-target overrides global)
	buildType := target.BuildType
	if buildType == "" {
		buildType = buildConfig.Type
	}
	bazelConfig := "release"
	if buildType == "Debug" || buildType == "debug" {
		bazelConfig = "debug"
	}

	// Create bazel repository cache directory inside project's .cache directory
	// This caches downloaded dependencies and repo mappings
	bazelRepoCacheDir := filepath.Join(absProjectRoot, ".cache", "ci", "bazel_repo_cache")
	if err := os.MkdirAll(bazelRepoCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create bazel repo cache directory: %w", err)
	}

	// Generate environment variable exports for the build script
	var envExports string
	if len(target.Env) > 0 {
		envExports = "# User-defined environment variables\n"
		for k, v := range target.Env {
			envExports += fmt.Sprintf("export %s=\"%s\"\n", k, v)
		}
	}

	// Create Bazel build script
	// Use --output_base to keep Bazel's output completely separate from the workspace
	// Use HOME=/root to reuse Bazel downloaded during Docker image build
	// Use --symlink_prefix=/dev/null to suppress symlinks (workspace is read-only)
	// Use --spawn_strategy=local to disable sandbox (causes issues in Docker)
	// Use --repository_cache to persist downloaded dependencies
	buildScript := fmt.Sprintf(`#!/bin/bash
set -e
%secho "  Building with Bazel..."
# Use HOME=/root to reuse Bazel pre-downloaded during Docker image build
export HOME=/root
BAZEL_OUTPUT_BASE=/bazel-cache
mkdir -p "$BAZEL_OUTPUT_BASE"
# Build with config
# --output_base: keep bazel output outside workspace
# --symlink_prefix=/dev/null: suppress symlinks (workspace is read-only)
# --spawn_strategy=local: disable sandbox (causes issues in Docker)
# --repository_cache: persist downloaded dependencies and repo state
bazel --output_base="$BAZEL_OUTPUT_BASE" build --config=%s --symlink_prefix=/dev/null --spawn_strategy=local --repository_cache=/bazel-repo-cache //...
echo "  Copying artifacts..."
mkdir -p /output/%s
# Copy only final executables (exclude object files, dep files, intermediate artifacts)
# Look for executables in bin directory, exclude common intermediate file patterns
find "$BAZEL_OUTPUT_BASE" -path "*/bin/*" -type f -executable \
    ! -name "*.o" ! -name "*.d" ! -name "*.a" ! -name "*.so" ! -name "*.dylib" \
    ! -name "*.runfiles*" ! -name "*.params" ! -name "*.sh" ! -name "*.py" \
    ! -name "*.repo_mapping" ! -name "*.cppmap" ! -name "MANIFEST" \
    ! -name "*.pic.o" ! -name "*.pic.d" \
    -exec cp {} /output/%s/ \; 2>/dev/null || true
# Copy only final libraries (static and shared), exclude pic intermediates
find "$BAZEL_OUTPUT_BASE" -path "*/bin/*" -type f \( -name "lib*.a" -o -name "lib*.so" \) \
    ! -name "*.pic.a" \
    -exec cp {} /output/%s/ \; 2>/dev/null || true
echo "  Build complete!"
`, envExports, bazelConfig, target.Name, target.Name, target.Name)

	// Run Docker container
	fmt.Printf("  %s Running Bazel build in Docker container...%s\n", Cyan, Reset)

	dockerArgs := []string{"run", "--rm"}
	// Add platform flag if specified (prevents warning on cross-platform runs)
	if target.Docker != nil && target.Docker.Platform != "" {
		dockerArgs = append(dockerArgs, "--platform", target.Docker.Platform)
	}

	// Mount workspace as read-only to prevent Bazel from creating files in it
	// Mount output directory separately
	// Mount bazel cache to a separate path
	// Mount bazel repo cache to a separate path
	dockerArgs = append(dockerArgs,
		"-v", absProjectRoot+":/workspace:ro",
		"-v", absOutputDir+":/output",
		"-v", bazelCacheDir+":/bazel-cache",
		"-v", bazelRepoCacheDir+":/bazel-repo-cache",
		"-w", "/workspace",
		imageName,
		"bash", "-c", buildScript)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker bazel build failed: %w", err)
	}

	return nil
}

// runDockerMesonBuildWithImage runs a Meson build inside Docker with specified image
func runDockerMesonBuildWithImage(target config.Toolchain, imageName, projectRoot, outputDir string, buildConfig config.ToolchainBuild) error {
	// Get absolute paths
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for project root: %w", err)
	}

	absOutputDir, err := filepath.Abs(filepath.Join(projectRoot, outputDir))
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output directory: %w", err)
	}

	// Create persistent build directory for caching
	hostBuildDir := filepath.Join(projectRoot, ".cache", "ci", target.Name)
	if err := os.MkdirAll(hostBuildDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}
	absBuildDir, err := filepath.Abs(hostBuildDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for build directory: %w", err)
	}

	// Determine build type (per-target overrides global)
	buildTypeConfig := target.BuildType
	if buildTypeConfig == "" {
		buildTypeConfig = buildConfig.Type
	}
	buildType := "release"
	if buildTypeConfig == "Debug" || buildTypeConfig == "debug" {
		buildType = "debug"
	}

	// Create subprojects directory if it doesn't exist to ensure it can be mounted
	hostSubprojectsDir := filepath.Join(projectRoot, "subprojects")
	if err := os.MkdirAll(hostSubprojectsDir, 0755); err != nil {
		return fmt.Errorf("failed to create subprojects directory: %w", err)
	}
	absSubprojectsDir, err := filepath.Abs(hostSubprojectsDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for subprojects directory: %w", err)
	}

	// Generate environment variable exports for the build script
	var envExports string
	if len(target.Env) > 0 {
		envExports = "# User-defined environment variables\n"
		for k, v := range target.Env {
			envExports += fmt.Sprintf("export %s=\"%s\"\n", k, v)
		}
	}

	// Build Meson arguments
	setupArgs := []string{"setup", "builddir", "--buildtype=" + buildType}

	// Add cross-file if triplet specified
	// Note: In cpx ci, the Docker image usually has the environment setup.
	// For Meson, we might need a cross-file if we are strictly cross-compiling not just running in a different arch container.
	// But usually 'cpx ci' uses an image that *is* the target environment (or emulated via QEMU).
	// So we typically don't need a cross file unless the image is a cross-compilation toolchain image.
	// For now, we assume the environment is correct or the image handles it.

	// Add custom Meson args
	setupArgs = append(setupArgs, buildConfig.MesonArgs...)

	// Build script
	// Mount host build dir to /workspace/builddir to persist subprojects and build artifacts
	// But /workspace is read-only. So we mount to /tmp/builddir and symlink or just build there.
	// Best approach: Mount host build dir to /tmp/builddir.
	// Meson needs source at /workspace.
	// We run meson setup from /workspace but point output to /tmp/builddir.

	// setupCmd := fmt.Sprintf("meson %s", strings.Join(setupArgs, " "))
	// compileCmd := "meson compile -C builddir"
	// if buildConfig.Verbose {
	// 	compileCmd += " -v"
	// }

	buildScript := fmt.Sprintf(`#!/bin/bash
set -e
%s# Ensure build directory exists (mounted from host)
mkdir -p /tmp/builddir

# Symlink /tmp/builddir to /workspace/builddir so Meson finds it where we expect,
# OR just tell meson to build in /tmp/builddir.
# Let's use /tmp/builddir directly.

echo "  Configuring Meson..."
# Run setup if build.ninja doesn't exist
if [ ! -f /tmp/builddir/build.ninja ]; then
    meson setup /tmp/builddir %s
else
    echo "  Build directory already configured, skipping setup."
fi

echo "  Building..."
meson compile -C /tmp/builddir

echo "  Copying artifacts..."
mkdir -p /workspace/out/%s

# Meson places executables in subdirectories (src/, bench/, etc.)
# Search in /tmp/builddir/src/ first (main executables)
if [ -d "/tmp/builddir/src" ]; then
    find /tmp/builddir/src -maxdepth 1 -type f -perm +111 ! -name "*.so" ! -name "*.dylib" ! -name "*.a" ! -name "*.p" ! -name "*_test" -exec cp {} /workspace/out/%s/ \; 2>/dev/null || true
fi

# Also check builddir root for executables
find /tmp/builddir -maxdepth 1 -type f -perm +111 ! -name "*.so" ! -name "*.dylib" ! -name "*.a" ! -name "*.p" ! -name "build.ninja" ! -name "*.json" -exec cp {} /workspace/out/%s/ \; 2>/dev/null || true

# Copy libraries from builddir and subdirectories
find /tmp/builddir -maxdepth 2 -type f \( -name "*.a" -o -name "*.so" -o -name "*.dylib" \) -exec cp {} /workspace/out/%s/ \; 2>/dev/null || true

# List what was copied
ls -la /workspace/out/%s/ 2>/dev/null || echo "  (no artifacts found)"

echo "  Build complete!"
`, envExports, strings.Join(setupArgs[2:], " "), target.Name, target.Name, target.Name, target.Name, target.Name)

	// Run Docker container
	fmt.Printf("  %s Running Meson build in Docker container...%s\n", Cyan, Reset)

	dockerArgs := []string{"run", "--rm"}
	// Add platform flag if specified (prevents warning on cross-platform runs)
	if target.Docker != nil && target.Docker.Platform != "" {
		dockerArgs = append(dockerArgs, "--platform", target.Docker.Platform)
	}

	// Mounts
	dockerArgs = append(dockerArgs,
		"-v", absProjectRoot+":/workspace:ro", // Source read-only
		"-v", absBuildDir+":/tmp/builddir", // Persistent build dir
		"-v", absSubprojectsDir+":/workspace/subprojects", // Subprojects read-write for downloading wraps
		"-v", absOutputDir+":/workspace/out", // Output dir
		"-w", "/workspace",
		imageName,
		"bash", "-c", buildScript)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker meson build failed: %w", err)
	}

	return nil
}

// runNativeBuild runs a native CMake build on the host system
func runNativeBuild(target config.Toolchain, projectRoot, outputDir string, buildConfig config.ToolchainBuild) error {
	// Detect project type and check for missing build tools
	projectType := DetectProjectType()
	missing := WarnMissingBuildTools(projectType)
	if len(missing) > 0 {
		fmt.Printf("  %sNote: Native build may fail due to missing tools%s\n", Yellow, Reset)
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

	// Add custom CMake args (per-target or global)
	cmakeArgs = append(cmakeArgs, cmakeOptions...)

	// Set environment variables from target config
	env := os.Environ()
	for k, v := range target.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Check if already configured
	ninjaFile := filepath.Join(absBuildDir, "build.ninja")
	if _, err := os.Stat(ninjaFile); os.IsNotExist(err) {
		fmt.Printf("  %s Configuring CMake (Ninja)...%s\n", Cyan, Reset)
		cmd := exec.Command("cmake", cmakeArgs...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cmake configure failed: %w", err)
		}
	} else {
		fmt.Printf("  %s Build directory already configured, skipping setup.%s\n", Green, Reset)
	}

	// Build
	fmt.Printf("  %s Building...%s\n", Cyan, Reset)
	buildArgs := []string{"--build", absBuildDir, "--config", buildType}
	if buildConfig.Jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", buildConfig.Jobs))
	}
	buildArgs = append(buildArgs, buildOptions...)

	cmd := exec.Command("cmake", buildArgs...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmake build failed: %w", err)
	}

	// Copy artifacts
	fmt.Printf("  %s Copying artifacts...%s\n", Cyan, Reset)

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

	fmt.Printf("  %s Build complete!%s\n", Green, Reset)
	return nil
}
