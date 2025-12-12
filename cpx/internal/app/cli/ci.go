package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ozacod/cpx/internal/app/cli/tui"
	"github.com/ozacod/cpx/pkg/config"
	"github.com/spf13/cobra"
)

// CICmd creates the ci command
func CICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Build for multiple targets using Docker (cross-compilation)",
		Long:  "Build for multiple targets using Docker (cross-compilation). Requires cpx.ci configuration file.",
		RunE:  runCI,
	}

	cmd.Flags().String("target", "", "Build only specific target (default: all)")
	cmd.Flags().Bool("rebuild", false, "Rebuild Docker images even if they exist")

	// Add add-target subcommand
	addTargetCmd := &cobra.Command{
		Use:   "add-target [target...]",
		Short: "Add a build target to cpx.ci",
		Long:  "Scan available Dockerfiles and add a build target to cpx.ci configuration.",
		RunE:  runAddTarget,
	}

	// Add list subcommand to add-target
	listTargetsCmd := &cobra.Command{
		Use:   "list",
		Short: "List all available build targets and select interactively",
		Long:  "List all available Dockerfiles and let you choose which to add to cpx.ci.",
		RunE:  runListTargets,
	}
	addTargetCmd.AddCommand(listTargetsCmd)
	cmd.AddCommand(addTargetCmd)

	return cmd
}

func runCI(cmd *cobra.Command, _ []string) error {
	target, _ := cmd.Flags().GetString("target")
	rebuild, _ := cmd.Flags().GetBool("rebuild")
	return runCICommand(target, rebuild)
}

// runAddTarget scans available Dockerfiles and adds selected targets to cpx.ci
func runAddTarget(_ *cobra.Command, args []string) error {
	// Get dockerfiles directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	dockerfilesDir := filepath.Join(homeDir, ".config", "cpx", "dockerfiles")

	// Check if directory exists
	if _, err := os.Stat(dockerfilesDir); os.IsNotExist(err) {
		return fmt.Errorf("dockerfiles directory not found: %s\n  Run 'cpx upgrade' to download Dockerfiles", dockerfilesDir)
	}

	// Scan for Dockerfile.* files
	entries, err := os.ReadDir(dockerfilesDir)
	if err != nil {
		return fmt.Errorf("failed to read dockerfiles directory: %w", err)
	}

	availableTargetsMap := make(map[string]bool)
	var availableTargets []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "Dockerfile.") && !strings.HasSuffix(name, ".example") {
			targetName := strings.TrimPrefix(name, "Dockerfile.")
			availableTargets = append(availableTargets, targetName)
			availableTargetsMap[targetName] = true
		}
	}

	if len(availableTargets) == 0 {
		return fmt.Errorf("no Dockerfiles found in %s", dockerfilesDir)
	}

	// Load existing cpx.ci or create new one
	ciConfig, err := config.LoadCI("cpx.ci")
	if err != nil {
		// Create new config
		ciConfig = &config.CIConfig{
			Targets: []config.CITarget{},
			Build: config.CIBuild{
				Type:         "Release",
				Optimization: "2",
				Jobs:         0,
			},
			Output: ".bin/ci",
		}
	}

	// Get existing target names to avoid duplicates
	existingTargets := make(map[string]bool)
	for _, t := range ciConfig.Targets {
		existingTargets[t.Name] = true
	}

	var selectedTargets []string

	// If args provided, use them directly
	if len(args) > 0 {
		for _, arg := range args {
			// Validate target exists
			if !availableTargetsMap[arg] {
				return fmt.Errorf("unknown target: %s\n  Available targets: %s", arg, strings.Join(availableTargets, ", "))
			}
			// Skip if already exists
			if existingTargets[arg] {
				fmt.Printf("%sTarget %s already in cpx.ci, skipping%s\n", Yellow, arg, Reset)
				continue
			}
			selectedTargets = append(selectedTargets, arg)
		}
	} else {
		// Interactive mode: filter out already added targets
		var newTargets []string
		for _, t := range availableTargets {
			if !existingTargets[t] {
				newTargets = append(newTargets, t)
			}
		}

		if len(newTargets) == 0 {
			fmt.Printf("%sAll available targets are already in cpx.ci%s\n", Yellow, Reset)
			return nil
		}

		// Print available targets for selection
		fmt.Printf("%sAvailable targets:%s\n", Cyan, Reset)
		for i, t := range newTargets {
			fmt.Printf("  %d. %s\n", i+1, t)
		}

		// Simple selection
		fmt.Printf("\n%sEnter target numbers to add (comma-separated, or 'all'):%s ", Cyan, Reset)
		var input string
		fmt.Scanln(&input)

		if strings.ToLower(strings.TrimSpace(input)) == "all" {
			selectedTargets = newTargets
		} else {
			// Parse comma-separated numbers
			parts := strings.Split(input, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				var idx int
				if _, err := fmt.Sscanf(part, "%d", &idx); err == nil {
					if idx >= 1 && idx <= len(newTargets) {
						selectedTargets = append(selectedTargets, newTargets[idx-1])
					}
				}
			}
		}
	}

	if len(selectedTargets) == 0 {
		fmt.Printf("%sNo targets selected%s\n", Yellow, Reset)
		return nil
	}

	// Add selected targets
	for _, targetName := range selectedTargets {
		target := deriveTargetConfig(targetName)
		ciConfig.Targets = append(ciConfig.Targets, target)
		fmt.Printf("%s+ Added target: %s%s\n", Green, targetName, Reset)
	}

	// Save cpx.ci
	if err := config.SaveCI(ciConfig, "cpx.ci"); err != nil {
		return err
	}

	fmt.Printf("\n%sSaved cpx.ci with %d target(s)%s\n", Green, len(ciConfig.Targets), Reset)
	return nil
}

// runListTargets shows all available targets and lets user select interactively
func runListTargets(_ *cobra.Command, _ []string) error {
	// Get dockerfiles directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	dockerfilesDir := filepath.Join(homeDir, ".config", "cpx", "dockerfiles")

	// Check if directory exists
	if _, err := os.Stat(dockerfilesDir); os.IsNotExist(err) {
		return fmt.Errorf("dockerfiles directory not found: %s\n  Run 'cpx upgrade' to download Dockerfiles", dockerfilesDir)
	}

	// Scan for Dockerfile.* files
	entries, err := os.ReadDir(dockerfilesDir)
	if err != nil {
		return fmt.Errorf("failed to read dockerfiles directory: %w", err)
	}

	var availableTargets []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "Dockerfile.") && !strings.HasSuffix(name, ".example") {
			targetName := strings.TrimPrefix(name, "Dockerfile.")
			availableTargets = append(availableTargets, targetName)
		}
	}

	if len(availableTargets) == 0 {
		return fmt.Errorf("no Dockerfiles found in %s", dockerfilesDir)
	}

	// Load existing cpx.ci to check which targets are already added
	ciConfig, err := config.LoadCI("cpx.ci")
	if err != nil {
		// Create new config
		ciConfig = &config.CIConfig{
			Targets: []config.CITarget{},
			Build: config.CIBuild{
				Type:         "Release",
				Optimization: "2",
				Jobs:         0,
			},
			Output: ".bin/ci",
		}
	}

	existingTargets := make(map[string]bool)
	for _, t := range ciConfig.Targets {
		existingTargets[t.Name] = true
	}

	// Build targets list for TUI
	var targets []tui.Target
	for _, name := range availableTargets {
		targets = append(targets, tui.Target{
			Name:        name,
			Platform:    describePlatform(name),
			AlreadyUsed: existingTargets[name],
		})
	}

	// Run interactive TUI
	selectedTargets, err := tui.RunTargetSelection(targets)
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if len(selectedTargets) == 0 {
		fmt.Printf("%sNo targets selected%s\n", Yellow, Reset)
		return nil
	}

	// Add selected targets
	for _, targetName := range selectedTargets {
		target := deriveTargetConfig(targetName)
		ciConfig.Targets = append(ciConfig.Targets, target)
		fmt.Printf("%s+ Added target: %s%s\n", Green, targetName, Reset)
	}

	// Save cpx.ci
	if err := config.SaveCI(ciConfig, "cpx.ci"); err != nil {
		return err
	}

	fmt.Printf("\n%sSaved cpx.ci with %d target(s)%s\n", Green, len(ciConfig.Targets), Reset)
	return nil
}

// describePlatform returns a human-readable platform description
func describePlatform(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return ""
	}
	os := parts[0]
	arch := parts[1]

	osNames := map[string]string{
		"linux":   "Linux",
		"windows": "Windows",
		"macos":   "macOS",
	}
	archNames := map[string]string{
		"amd64": "x86_64",
		"arm64": "ARM64",
	}

	osName := osNames[os]
	if osName == "" {
		osName = os
	}
	archName := archNames[arch]
	if archName == "" {
		archName = arch
	}

	return osName + " " + archName
}

// dimStyle applies dim styling to text
func dimStyle(s string) string {
	return Dim + s + Reset
}

// deriveTargetConfig derives a CITarget from a target name
func deriveTargetConfig(name string) config.CITarget {
	target := config.CITarget{
		Name:       name,
		Dockerfile: "Dockerfile." + name,
		Image:      "cpx-" + name,
	}

	// Derive triplet and platform from name
	parts := strings.Split(name, "-")
	if len(parts) >= 2 {
		os := parts[0]   // linux, windows, macos
		arch := parts[1] // amd64, arm64

		switch os {
		case "linux":
			if arch == "amd64" {
				target.Triplet = "x64-linux"
				target.Platform = "linux/amd64"
			} else if arch == "arm64" {
				target.Triplet = "arm64-linux"
				target.Platform = "linux/arm64"
			}
		case "windows":
			if arch == "amd64" {
				target.Triplet = "x64-mingw-static"
				target.Platform = "linux/amd64" // Cross-compile from Linux
			}
		case "macos":
			if arch == "amd64" {
				target.Triplet = "x64-osx"
				target.Platform = "linux/amd64" // Requires osxcross
			} else if arch == "arm64" {
				target.Triplet = "arm64-osx"
				target.Platform = "linux/arm64"
			}
		}
	}

	return target
}

var ciCommandExecuted = false

func runCICommand(targetName string, rebuild bool) error {
	if ciCommandExecuted {
		fmt.Printf("%s[DEBUG] CI command already executed in this process (PID: %d), skipping second invocation.%s\n", Yellow, os.Getpid(), Reset)
		return nil
	}
	ciCommandExecuted = true
	// fmt.Printf("%s[DEBUG] Starting CI build (PID: %d)%s\n", Yellow, os.Getpid(), Reset)

	// Load cpx.ci configuration
	ciConfig, err := config.LoadCI("cpx.ci")
	if err != nil {
		return fmt.Errorf("failed to load cpx.ci: %w\n  Create cpx.ci file or run 'cpx build' for local builds", err)
	}

	// Filter targets if specific target requested
	targets := ciConfig.Targets
	if targetName != "" {
		found := false
		for _, t := range ciConfig.Targets {
			if t.Name == targetName {
				targets = []config.CITarget{t}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("target '%s' not found in cpx.ci", targetName)
		}
	}

	if len(targets) == 0 {
		return fmt.Errorf("no targets defined in cpx.ci")
	}

	// Get Dockerfiles directory from config
	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}
	dockerfilesDir := filepath.Join(configDir, "dockerfiles")

	// Get absolute path to dockerfiles directory
	absDockerfilesDir, err := filepath.Abs(dockerfilesDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute dockerfiles directory: %w", err)
	}

	// Verify dockerfiles directory exists
	if _, err := os.Stat(absDockerfilesDir); os.IsNotExist(err) {
		return fmt.Errorf("dockerfiles directory not found: %s\n  Run 'cpx upgrade' to download Dockerfiles", absDockerfilesDir)
	}

	// Create output directory
	outputDir := ciConfig.Output
	if outputDir == "" {
		outputDir = filepath.Join(".bin", "ci")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("%s Building for %d target(s) using Docker...%s\n", Cyan, len(targets), Reset)

	// Get project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	// Build and run for each target
	for i, target := range targets {
		fmt.Printf("\n%s[%d/%d] Building target: %s%s\n", Cyan, i+1, len(targets), target.Name, Reset)

		// Build Docker image
		dockerfilePath := filepath.Join(absDockerfilesDir, target.Dockerfile)
		if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
			return fmt.Errorf("dockerfile not found: %s\n  Run 'cpx upgrade' to download Dockerfiles", dockerfilePath)
		}

		if err := buildDockerImage(dockerfilePath, target.Image, target.Platform, rebuild); err != nil {
			return fmt.Errorf("failed to build Docker image %s: %w", target.Image, err)
		}

		// Run build in Docker container
		if err := runDockerBuild(target, projectRoot, outputDir, ciConfig.Build); err != nil {
			return fmt.Errorf("failed to build target %s: %w", target.Name, err)
		}

		fmt.Printf("%s Target %s built successfully%s\n", Green, target.Name, Reset)
	}

	fmt.Printf("\n%s All targets built successfully!%s\n", Green, Reset)
	fmt.Printf("   Artifacts are in: %s\n", outputDir)
	return nil
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for project markers
	for {
		// Check for cpx.ci or CMakeLists.txt or MODULE.bazel (project markers)
		if _, err := os.Stat(filepath.Join(dir, "cpx.ci")); err == nil {
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

func buildDockerImage(dockerfilePath, imageName, platform string, rebuild bool) error {
	// Check if image already exists
	if !rebuild {
		cmd := exec.Command("docker", "images", "-q", imageName)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			fmt.Printf("  %s Docker image %s already exists%s\n", Green, imageName, Reset)
			return nil
		}
	}

	fmt.Printf("  %s Building Docker image: %s...%s\n", Cyan, imageName, Reset)

	// Get absolute paths
	absDockerfilePath, err := filepath.Abs(dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute dockerfile path: %w", err)
	}

	// Get directory containing the Dockerfile (build context)
	dockerfileDir, err := filepath.Abs(filepath.Dir(dockerfilePath))
	if err != nil {
		return fmt.Errorf("failed to get absolute dockerfile directory: %w", err)
	}

	// Verify Dockerfile exists
	if _, err := os.Stat(absDockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found: %s", absDockerfilePath)
	}

	// Build Docker image with platform flag if specified
	// Use buildx for better multi-arch support
	buildArgs := []string{"buildx", "build", "-f", absDockerfilePath, "-t", imageName}
	if platform != "" {
		buildArgs = append(buildArgs, "--platform", platform)
	}
	buildArgs = append(buildArgs, "--load") // Load into local Docker daemon
	buildArgs = append(buildArgs, dockerfileDir)

	cmd := exec.Command("docker", buildArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// If buildx fails, fall back to regular docker build
	if err := cmd.Run(); err != nil {
		fmt.Printf("  %s  docker buildx failed, trying regular docker build...%s\n", Yellow, Reset)
		// Fallback to regular docker build
		buildArgs = []string{"build", "-f", absDockerfilePath, "-t", imageName}
		if platform != "" {
			buildArgs = append(buildArgs, "--platform", platform)
		}
		buildArgs = append(buildArgs, dockerfileDir)

		cmd = exec.Command("docker", buildArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker build failed: %w", err)
		}
	}

	fmt.Printf("  %s Docker image %s built successfully%s\n", Green, imageName, Reset)
	return nil
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

func runDockerBuild(target config.CITarget, projectRoot, outputDir string, buildConfig config.CIBuild) error {
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
		return runDockerBazelBuild(target, projectRoot, outputDir, buildConfig)
	}

	// Check if this is a Meson project
	if _, err := os.Stat(filepath.Join(projectRoot, "meson.build")); err == nil {
		return runDockerMesonBuild(target, projectRoot, outputDir, buildConfig)
	}

	// Detect project type (executable or library) for CMake projects
	isExe, err := detectProjectType(projectRoot)
	if err != nil {
		// If we can't detect, default to executable
		isExe = true
	}

	// vcpkg is installed in the Docker images at /opt/vcpkg
	// No need to mount from host - images are self-contained

	// Determine build type and optimization
	buildType := buildConfig.Type
	if buildType == "" {
		buildType = "Release"
	}

	optLevel := buildConfig.Optimization
	if optLevel == "" {
		optLevel = "2"
	}

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

	// Add triplet if specified - this is critical for cross-compilation
	triplet := target.Triplet
	// The triplet should match the Docker image being used

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

	if triplet != "" {
		cmakeArgs = append(cmakeArgs, "-DVCPKG_TARGET_TRIPLET="+triplet)
		// VCPKG_HOST_TRIPLET should match the container architecture
		// It's automatically detected by vcpkg based on the container's architecture
		// No need to set it explicitly - vcpkg will detect it correctly from the container
	}

	// Disable registry updates via CMake variable
	// This is more reliable than environment variables
	cmakeArgs = append(cmakeArgs, "-DVCPKG_DISABLE_REGISTRY_UPDATE=ON")

	// Add custom CMake args
	cmakeArgs = append(cmakeArgs, buildConfig.CMakeArgs...)

	// Build command arguments
	buildArgs := []string{"--build", containerBuildDir, "--config", buildType}
	if buildConfig.Jobs > 0 {
		buildArgs = append(buildArgs, "--parallel", fmt.Sprintf("%d", buildConfig.Jobs))
	}
	buildArgs = append(buildArgs, buildConfig.BuildArgs...)

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

	// Bash build script for Linux/macOS
	buildScript := fmt.Sprintf(`#!/bin/bash
set -e
export VCPKG_ROOT=/opt/vcpkg
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
`, vcpkgInstalledPath, vcpkgDownloadsPath, vcpkgBuildtreesPath, binaryCachePath, binaryCachePath, containerBuildDir, containerBuildDir, strings.Join(cmakeArgs, " "), strings.Join(buildArgs, " "), target.Name, copyCommand)

	// Run Docker container
	fmt.Printf("  %s Running build in Docker container...%s\n", Cyan, Reset)

	// Use platform from target config
	platform := target.Platform

	// Mount only necessary directories:
	// - Source code (read-only to avoid modifying host files)
	// - Build directory (for caching CMake build artifacts) - mount to a subdirectory that can be created
	// - Output directory (for artifacts)
	// - vcpkg cache directory (from build/.vcpkg_cache to /tmp/.vcpkg_cache)
	dockerArgs := []string{"run", "--rm"}
	if platform != "" {
		dockerArgs = append(dockerArgs, "--platform", platform)
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
		target.Image,
		command, "-c", buildScript)

	cmd := exec.Command("docker", dockerArgs...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker run failed: %w", err)
	}

	return nil
}

// runDockerBazelBuild runs a Bazel build inside Docker
func runDockerBazelBuild(target config.CITarget, projectRoot, outputDir string, buildConfig config.CIBuild) error {
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

	// Determine build config
	bazelConfig := "release"
	if buildConfig.Type == "Debug" || buildConfig.Type == "debug" {
		bazelConfig = "debug"
	}

	// Create bazel repository cache directory inside project's .cache directory
	// This caches downloaded dependencies and repo mappings
	bazelRepoCacheDir := filepath.Join(absProjectRoot, ".cache", "ci", "bazel_repo_cache")
	if err := os.MkdirAll(bazelRepoCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create bazel repo cache directory: %w", err)
	}

	// Create Bazel build script
	// Use --output_base to keep Bazel's output completely separate from the workspace
	// Use HOME=/root to reuse Bazel downloaded during Docker image build
	// Use --symlink_prefix=/dev/null to suppress symlinks (workspace is read-only)
	// Use --spawn_strategy=local to disable sandbox (causes issues in Docker)
	// Use --repository_cache to persist downloaded dependencies
	buildScript := fmt.Sprintf(`#!/bin/bash
set -e
echo "  Building with Bazel..."
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
`, bazelConfig, target.Name, target.Name, target.Name)

	// Run Docker container
	fmt.Printf("  %s Running Bazel build in Docker container...%s\n", Cyan, Reset)

	platform := target.Platform
	dockerArgs := []string{"run", "--rm"}
	if platform != "" {
		dockerArgs = append(dockerArgs, "--platform", platform)
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
		target.Image,
		"bash", "-c", buildScript)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker bazel build failed: %w", err)
	}

	return nil
}

// runDockerMesonBuild runs a Meson build inside Docker
func runDockerMesonBuild(target config.CITarget, projectRoot, outputDir string, buildConfig config.CIBuild) error {
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

	// Determine build type
	buildType := "release"
	if buildConfig.Type == "Debug" || buildConfig.Type == "debug" {
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
# Ensure build directory exists (mounted from host)
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
`, strings.Join(setupArgs[2:], " "), target.Name, target.Name, target.Name, target.Name, target.Name)

	// Run Docker container
	fmt.Printf("  %s Running Meson build in Docker container...%s\n", Cyan, Reset)

	platform := target.Platform
	dockerArgs := []string{"run", "--rm"}
	if platform != "" {
		dockerArgs = append(dockerArgs, "--platform", platform)
	}

	// Mounts
	dockerArgs = append(dockerArgs,
		"-v", absProjectRoot+":/workspace:ro", // Source read-only
		"-v", absBuildDir+":/tmp/builddir", // Persistent build dir
		"-v", absSubprojectsDir+":/workspace/subprojects", // Subprojects read-write for downloading wraps
		"-v", absOutputDir+":/workspace/out", // Output dir
		"-w", "/workspace",
		target.Image,
		"bash", "-c", buildScript)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker meson build failed: %w", err)
	}

	return nil
}
