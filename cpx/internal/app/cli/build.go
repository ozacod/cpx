package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/ozacod/cpx/internal/pkg/vcpkg"
	"github.com/spf13/cobra"
)

// BuildCmd creates the build command
func BuildCmd(client *vcpkg.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compile the project",
		Long: `Compile the project. Automatically detects project type:
  - vcpkg/CMake projects: Uses CMake with vcpkg toolchain
  - Bazel projects: Uses bazel build`,
		Example: `  cpx build              # Debug build (default)
  cpx build --release    # Release build (-O2)
  cpx build -O3          # Maximum optimization
  cpx build -j 8         # Use 8 parallel jobs
  cpx build --clean      # Clean rebuild
  cpx build --asan       # Build with AddressSanitizer
  cpx build --tsan       # Build with ThreadSanitizer
  cpx build all          # Build all toolchains (Docker)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd, args, client)
		},
	}

	cmd.Flags().BoolP("release", "r", false, "Release build (-O2). Default is debug")
	cmd.Flags().Bool("debug", false, "Debug build (-O0). Default; kept for compatibility")
	cmd.Flags().IntP("jobs", "j", 0, "Parallel jobs for build (0 = auto)")
	cmd.Flags().String("toolchain", "", "Toolchain to build (from cpx-ci.yaml)")
	cmd.Flags().BoolP("clean", "c", false, "Clean build directory before building")
	cmd.Flags().StringP("opt", "O", "", "Override optimization level: 0,1,2,3,s,fast")
	cmd.Flags().Bool("verbose", false, "Show full build output")
	// Sanitizer flags
	cmd.Flags().Bool("asan", false, "Build with AddressSanitizer")
	cmd.Flags().Bool("tsan", false, "Build with ThreadSanitizer")
	cmd.Flags().Bool("msan", false, "Build with MemorySanitizer")
	cmd.Flags().Bool("ubsan", false, "Build with UndefinedBehaviorSanitizer")
	cmd.Flags().Bool("list", false, "List available build targets")

	// Add 'all' subcommand for building all toolchains
	allCmd := &cobra.Command{
		Use:   "all",
		Short: "Build all toolchains using Docker",
		Long:  "Build for all toolchains defined in cpx-ci.yaml using Docker containers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			toolchain, _ := cmd.Flags().GetString("toolchain")
			rebuild, _ := cmd.Flags().GetBool("rebuild")
			return runToolchainBuild(toolchain, rebuild, false)
		},
	}
	allCmd.Flags().String("toolchain", "", "Build only specific toolchain (default: all)")
	allCmd.Flags().Bool("rebuild", false, "Rebuild Docker images even if they exist")
	cmd.AddCommand(allCmd)

	return cmd
}

func runBuild(cmd *cobra.Command, _ []string, client *vcpkg.Client) error {
	release, _ := cmd.Flags().GetBool("release")
	jobs, _ := cmd.Flags().GetInt("jobs")
	toolchain, _ := cmd.Flags().GetString("toolchain")
	clean, _ := cmd.Flags().GetBool("clean")
	optLevel, _ := cmd.Flags().GetString("opt")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// --toolchain is for CI builds (Docker)
	// Use `cpx build all --toolchain <name>` for the same behavior
	if toolchain != "" {
		return runToolchainBuild(toolchain, false, false)
	}

	// Parse sanitizer flags
	asan, _ := cmd.Flags().GetBool("asan")
	tsan, _ := cmd.Flags().GetBool("tsan")
	msan, _ := cmd.Flags().GetBool("msan")
	ubsan, _ := cmd.Flags().GetBool("ubsan")

	// Validate only one sanitizer is used
	sanitizer := ""
	sanitizerCount := 0
	if asan {
		sanitizer = "asan"
		sanitizerCount++
	}
	if tsan {
		sanitizer = "tsan"
		sanitizerCount++
	}
	if msan {
		sanitizer = "msan"
		sanitizerCount++
	}
	if ubsan {
		sanitizer = "ubsan"
		sanitizerCount++
	}
	if sanitizerCount > 1 {
		return fmt.Errorf("only one sanitizer can be used at a time (got %d)", sanitizerCount)
	}

	projectType := DetectProjectType()

	// Check for missing build tools and warn the user
	WarnMissingBuildTools(projectType)

	list, _ := cmd.Flags().GetBool("list")

	switch projectType {
	case ProjectTypeBazel:
		if list {
			return listBazelTargets()
		}
		return runBazelBuild(release, "", clean, verbose, optLevel, sanitizer)
	case ProjectTypeMeson:
		if list {
			return listMesonTargets()
		}
		return runMesonBuild(release, "", clean, verbose, optLevel, sanitizer)
	case ProjectTypeVcpkg:
		if list {
			return listCMakeTargets(release, optLevel, client)
		}
		return build.BuildProject(release, jobs, "", clean, optLevel, verbose, sanitizer, client)
	default:
		// Fall back to CMake build even without vcpkg.json
		if list {
			return listCMakeTargets(release, optLevel, client)
		}
		return build.BuildProject(release, jobs, "", clean, optLevel, verbose, sanitizer, client)
	}
}

func listBazelTargets() error {
	fmt.Printf("%sListing Bazel targets...%s\n", Cyan, Reset)
	// Query for all targets of type rule
	cmd := execCommand("bazel", "query", "//...", "--output", "label_kind")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func listMesonTargets() error {
	buildDir := "builddir"
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		return fmt.Errorf("build directory '%s' does not exist. Run 'cpx build' first to configure the project", buildDir)
	}

	fmt.Printf("%sListing Meson targets...%s\n", Cyan, Reset)

	// Use meson introspect to get targets as JSON, then format nicely
	cmd := execCommand("meson", "introspect", "--targets", buildDir)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to introspect meson targets: %w", err)
	}

	// Parse JSON output and display nicely
	// Format: [{"name": "target", "type": "executable", ...}, ...]
	type MesonTarget struct {
		Name     string   `json:"name"`
		Type     string   `json:"type"`
		Filename []string `json:"filename"`
	}

	var targets []MesonTarget
	if err := json.Unmarshal(output, &targets); err != nil {
		// If JSON parsing fails, just print the raw output
		fmt.Println(string(output))
		return nil
	}

	fmt.Printf("Targets in %s:\n", buildDir)
	for _, t := range targets {
		fmt.Printf("  %s (%s)\n", t.Name, t.Type)
	}

	return nil
}

func listCMakeTargets(release bool, optLevel string, client *vcpkg.Client) error {
	fmt.Printf("%sListing CMake targets...%s\n", Cyan, Reset)

	// `cpx` builds into `.cache/native/<config>`
	cacheDir := ".cache/native"

	// Determine preferred build directory based on flags
	preferredConfig := "debug"
	if release {
		preferredConfig = "release"
	}
	if optLevel != "" {
		preferredConfig = "O" + optLevel
	}

	// Try preferred config first, then fall back to any available
	preferredDir := filepath.Join(cacheDir, preferredConfig)
	if _, err := os.Stat(preferredDir); err == nil {
		return listTargetsInDir(preferredDir)
	}

	// Fall back to any available build directory
	entries, err := os.ReadDir(cacheDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				bDir := filepath.Join(cacheDir, e.Name())
				if err := listTargetsInDir(bDir); err == nil {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("no configured build directory found. Please run 'cpx build' first to configure the project")
}

// listTargetsInDir lists user-defined targets in a specific build directory.
func listTargetsInDir(bDir string) error {
	// Check for Ninja build
	if _, err := os.Stat(filepath.Join(bDir, "build.ninja")); err == nil {
		// Use ninja -t targets for complete target info
		cmd := execCommand("ninja", "-C", bDir, "-t", "targets", "all")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return err
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

		if len(userTargets) > 0 {
			fmt.Printf("Targets in %s:\n", bDir)
			for _, t := range userTargets {
				fmt.Printf("  %s\n", t)
			}
		} else {
			fmt.Printf("No user-defined targets found in %s\n", bDir)
		}
		return nil
	}

	// Fallback for Make builds
	if isMakefile(filepath.Join(bDir, "Makefile")) {
		cmd := execCommand("cmake", "--build", bDir, "--target", "help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}

		lines := strings.Split(string(output), "\n")
		var userTargets []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if targetName, isUser := parseMakeTarget(line); isUser {
				userTargets = append(userTargets, targetName)
			}
		}

		if len(userTargets) > 0 {
			fmt.Printf("Targets in %s:\n", bDir)
			for _, t := range userTargets {
				fmt.Printf("  %s\n", t)
			}
		} else {
			fmt.Printf("No user-defined targets found in %s\n", bDir)
		}
		return nil
	}

	return fmt.Errorf("no build system found in %s", bDir)
}

func isMakefile(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseNinjaTarget parses a line from ninja -t targets output
// and returns the target name if it's a user-defined target (executable/library).
// Ninja format: "target_name: target_type"
// Examples:
//   - "ffff-cmake: CXX_EXECUTABLE_LINKER__ffff-cmake_" -> ("ffff-cmake", true)
//   - "mylib: CXX_STATIC_LIBRARY_LINKER__mylib_" -> ("mylib", true)
//   - "edit_cache: phony" -> ("", false) - CMake internal
//   - "all: phony" -> ("", false) - CMake internal
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
	// Executables: CXX_EXECUTABLE_LINKER__*, C_EXECUTABLE_LINKER__*
	// Static libs: CXX_STATIC_LIBRARY_LINKER__*, C_STATIC_LIBRARY_LINKER__*
	// Shared libs: CXX_SHARED_LIBRARY_LINKER__*, C_SHARED_LIBRARY_LINKER__*
	isExecutable := strings.Contains(targetType, "EXECUTABLE_LINKER")
	isLibrary := strings.Contains(targetType, "LIBRARY_LINKER")

	if isExecutable || isLibrary {
		return targetName, true
	}

	return "", false
}

// parseMakeTarget parses a line from cmake --build --target help output for Makefile builds
// and returns the target name if it's a user-defined target.
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

func runBazelBuild(release bool, target string, clean bool, verbose bool, optLevel string, sanitizer string) error {
	// Clean if requested
	if clean {
		fmt.Printf("%sCleaning Bazel build...%s\n", Cyan, Reset)
		cleanCmd := execCommand("bazel", "clean")
		cleanCmd.Stdout = os.Stdout
		cleanCmd.Stderr = os.Stderr
		if err := cleanCmd.Run(); err != nil {
			return fmt.Errorf("bazel clean failed: %w", err)
		}
		// Also remove build directory
		os.RemoveAll("build")
	}

	// Build args
	bazelArgs := []string{"build"}

	// Handle optimization level - optLevel takes precedence over release flag
	var optLabel string
	switch optLevel {
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
		if release {
			bazelArgs = append(bazelArgs, "--config=release")
			optLabel = "release"
		} else {
			bazelArgs = append(bazelArgs, "--config=debug")
			optLabel = "debug"
		}
	}

	// Add sanitizer flags
	if sanitizer != "" {
		switch sanitizer {
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
	if target != "" {
		bazelArgs = append(bazelArgs, target)
	} else {
		bazelArgs = append(bazelArgs, "//...")
	}

	fmt.Printf("%sBuilding with Bazel [%s]...%s\n", Cyan, optLabel, Reset)
	if verbose {
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
	outDirName := "debug"
	if optLevel != "" {
		outDirName = "O" + optLevel
	} else if release {
		outDirName = "release"
	}
	outputDir := filepath.Join(".bin", "native", outDirName)

	// Copy artifacts to build/<config>/ directory
	// Remove existing build artifacts for this config first
	os.RemoveAll(outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Copy executables and libraries from bazel-bin to build/<config>/
	fmt.Printf("%sCopying artifacts to %s/...%s\n", Cyan, outputDir, Reset)

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

	fmt.Printf("%s✓ Build successful%s\n", Green, Reset)
	fmt.Printf("  Artifacts in: %s/\n", outputDir)
	return nil
}

func runMesonBuild(release bool, target string, clean bool, verbose bool, optLevel string, sanitizer string) error {
	buildDir := "builddir"

	// Determine build type and optimization from flags
	// Determine build type and optimization from flags
	var buildType, optimization, optLabel string

	switch optLevel {
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
		if release {
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
	if sanitizer != "" {
		optLabel += "+" + sanitizer
	}

	// Clean if requested or if optimization changed
	if clean {
		fmt.Printf("%sCleaning Meson build...%s\n", Cyan, Reset)
		os.RemoveAll(buildDir)
	}

	// Check if build directory exists (needs setup)
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		fmt.Printf("%sSetting up Meson build directory [%s]...%s\n", Cyan, optLabel, Reset)
		setupArgs := []string{"setup", buildDir}
		setupArgs = append(setupArgs, "--buildtype="+buildType)
		setupArgs = append(setupArgs, "--optimization="+optimization)
		if optLevel == "fast" {
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
		fmt.Printf("%sReconfiguring Meson [%s]...%s\n", Cyan, optLabel, Reset)
		reconfigArgs := []string{"configure", buildDir}
		reconfigArgs = append(reconfigArgs, "--buildtype="+buildType)
		reconfigArgs = append(reconfigArgs, "--optimization="+optimization)
		if optLevel == "fast" {
			reconfigArgs = append(reconfigArgs, "-Dc_args=-ffast-math", "-Dcpp_args=-ffast-math")
		}
		reconfigCmd := execCommand("meson", reconfigArgs...)
		reconfigCmd.Stdout = os.Stdout
		reconfigCmd.Stderr = os.Stderr
		// Ignore reconfigure errors - may fail if no changes needed
		_ = reconfigCmd.Run()
	}

	// Build
	fmt.Printf("%sBuilding with Meson...%s\n", Cyan, Reset)
	compileArgs := []string{"compile", "-C", buildDir}
	if target != "" {
		compileArgs = append(compileArgs, target)
	}
	if verbose {
		compileArgs = append(compileArgs, "-v")
	}
	buildCmd := execCommand("meson", compileArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("meson compile failed: %w", err)
	}

	// Determine output directory based on config
	outDirName := "debug"
	if optLevel != "" {
		outDirName = "O" + optLevel
	} else if release {
		outDirName = "release"
	}
	outputDir := filepath.Join(".bin", "native", outDirName)

	// Copy artifacts to output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("%sCopying artifacts to %s/...%s\n", Cyan, outputDir, Reset)
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

	fmt.Printf("%s✓ Build successful%s\n", Green, Reset)
	fmt.Printf("  Artifacts in: %s/\n", outputDir)
	return nil
}
