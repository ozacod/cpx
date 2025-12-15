package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ozacod/cpx/internal/app/cli/tui"
	"github.com/ozacod/cpx/internal/pkg/git"
	"github.com/ozacod/cpx/internal/pkg/templates"
	"github.com/ozacod/cpx/internal/pkg/vcpkg"
	"github.com/spf13/cobra"
)

// NewCmd creates the new command with interactive TUI
func NewCmd(client *vcpkg.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new C++ project (interactive)",
		Long:  "Create a new C++ project using an interactive TUI. This will guide you through the project configuration.",
		Example: `  cpx new            # launch the interactive creator
  cpx new --help    # view options`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args, client)
		},
		Args: cobra.NoArgs,
	}

	return cmd
}

func runNew(_ *cobra.Command, _ []string, client *vcpkg.Client) error {
	// Initialize and run the TUI
	p := tea.NewProgram(tui.InitialModel())
	m, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Get the final model
	finalModel, ok := m.(tui.Model)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	// Check if cancelled
	if finalModel.IsCancelled() {
		return nil
	}

	// Get the configuration
	config := finalModel.GetConfig()

	// Create the project with the configuration
	return createProjectFromTUI(config, client)
}

func createProjectFromTUI(config tui.ProjectConfig, vcpkgClient *vcpkg.Client) error {
	projectName := config.Name

	// Check if directory already exists
	if _, err := os.Stat(projectName); err == nil {
		return fmt.Errorf("directory '%s' already exists", projectName)
	}

	// Create the new directory
	if err := os.MkdirAll(projectName, 0755); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", projectName, err)
	}

	// Build configuration from TUI choices
	cfg := &tui.ProjectConfig{
		Name:           projectName,
		IsLibrary:      config.IsLibrary,
		CppStandard:    config.CppStandard,
		TestFramework:  config.TestFramework,
		ClangFormat:    config.ClangFormat,
		PackageManager: config.PackageManager,
		VCS:            config.VCS,
		UseHooks:       config.UseHooks,
		GitHooks:       config.GitHooks,
		PreCommit:      config.PreCommit,
		PrePush:        config.PrePush,
		Benchmark:      config.Benchmark,
	}

	// Set hooks
	if len(config.GitHooks) > 0 {
		for _, hook := range config.GitHooks {
			if hook == "fmt" || hook == "lint" {
				cfg.PreCommit = append(cfg.PreCommit, hook)
			}
			if hook == "test" {
				cfg.PrePush = append(cfg.PrePush, hook)
			}
		}
	}

	// Set VCS configuration defaults
	if cfg.VCS == "" {
		cfg.VCS = "git"
	}

	// Set PackageManager configuration defaults
	if cfg.PackageManager == "" {
		cfg.PackageManager = "vcpkg"
	}

	// Initialize git repository only if VCS is set to git
	if cfg.VCS == "git" {
		cmd := exec.Command("git", "init")
		cmd.Dir = projectName
		_ = cmd.Run() // Ignore errors silently
	}

	// Set C++ standard default
	cppStandard := cfg.CppStandard
	if cppStandard == 0 {
		cppStandard = 17
	}

	projectVersion := "0.1.0"

	// Generate benchmark artifacts if enabled
	benchSources, _ := templates.GenerateBenchmarkSources(projectName, cfg.Benchmark)

	// Create directory structure
	dirs := []string{
		"include/" + projectName,
		"src",
		"tests",
		"scripts",
		"docs",
	}
	if benchSources != nil {
		dirs = append(dirs, "bench")
	}
	for _, dir := range dirs {
		dirPath := filepath.Join(projectName, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", dirPath, err)
		}
	}

	// Generate build system files based on package manager choice
	if cfg.PackageManager == "bazel" {
		// Generate MODULE.bazel
		moduleBazel := templates.GenerateModuleBazel(projectName, projectVersion, cfg.TestFramework, cfg.Benchmark)
		if err := os.WriteFile(filepath.Join(projectName, "MODULE.bazel"), []byte(moduleBazel), 0644); err != nil {
			return fmt.Errorf("failed to write MODULE.bazel: %w", err)
		}

		// Generate root BUILD.bazel (aliases)
		buildBazel := templates.GenerateBuildBazelRoot(projectName, !cfg.IsLibrary)
		if err := os.WriteFile(filepath.Join(projectName, "BUILD.bazel"), []byte(buildBazel), 0644); err != nil {
			return fmt.Errorf("failed to write BUILD.bazel: %w", err)
		}

		// Generate src/BUILD.bazel
		srcBuild := templates.GenerateBuildBazelSrc(projectName, !cfg.IsLibrary)
		if err := os.WriteFile(filepath.Join(projectName, "src/BUILD.bazel"), []byte(srcBuild), 0644); err != nil {
			return fmt.Errorf("failed to write src/BUILD.bazel: %w", err)
		}

		// Generate include/BUILD.bazel
		includeBuild := templates.GenerateBuildBazelInclude(projectName)
		if err := os.WriteFile(filepath.Join(projectName, "include/BUILD.bazel"), []byte(includeBuild), 0644); err != nil {
			return fmt.Errorf("failed to write include/BUILD.bazel: %w", err)
		}

		// Generate .bazelrc
		bazelrc := templates.GenerateBazelrc(cppStandard)
		if err := os.WriteFile(filepath.Join(projectName, ".bazelrc"), []byte(bazelrc), 0644); err != nil {
			return fmt.Errorf("failed to write .bazelrc: %w", err)
		}

		// Generate .bazelignore
		bazelignore := templates.GenerateBazelignore()
		if err := os.WriteFile(filepath.Join(projectName, ".bazelignore"), []byte(bazelignore), 0644); err != nil {
			return fmt.Errorf("failed to write .bazelignore: %w", err)
		}
	} else if cfg.PackageManager == "meson" {
		// Generate meson.build (root)
		mesonBuild := templates.GenerateMesonBuildRoot(projectName, !cfg.IsLibrary, cppStandard, cfg.TestFramework, cfg.Benchmark)
		if err := os.WriteFile(filepath.Join(projectName, "meson.build"), []byte(mesonBuild), 0644); err != nil {
			return fmt.Errorf("failed to write meson.build: %w", err)
		}

		// Generate src/meson.build
		srcMeson := templates.GenerateMesonBuildSrc(projectName, !cfg.IsLibrary)
		if err := os.WriteFile(filepath.Join(projectName, "src/meson.build"), []byte(srcMeson), 0644); err != nil {
			return fmt.Errorf("failed to write src/meson.build: %w", err)
		}

		// Generate meson_options.txt (use _options.txt for wider compatibility)
		mesonOptions := templates.GenerateMesonOptions()
		if err := os.WriteFile(filepath.Join(projectName, "meson_options.txt"), []byte(mesonOptions), 0644); err != nil {
			return fmt.Errorf("failed to write meson_options.txt: %w", err)
		}

		// Create subprojects directory for wraps
		if err := os.MkdirAll(filepath.Join(projectName, "subprojects"), 0755); err != nil {
			return fmt.Errorf("failed to create subprojects directory: %w", err)
		}

		// Download required wrap files for test framework
		if cfg.TestFramework != "" && cfg.TestFramework != "none" {
			wrapName := ""
			switch cfg.TestFramework {
			case "googletest":
				wrapName = "gtest"
			case "catch2":
				wrapName = "catch2"
			case "doctest":
				wrapName = "doctest"
			}
			if wrapName != "" {
				if err := downloadMesonWrap(projectName, wrapName); err != nil {
					fmt.Printf("%sWarning: could not download %s wrap: %v%s\n", Yellow, wrapName, err, Reset)
				}
			}
		}

		// Download required wrap files for benchmark framework
		if cfg.Benchmark != "" && cfg.Benchmark != "none" {
			wrapName := ""
			switch cfg.Benchmark {
			case "google-benchmark":
				wrapName = "google-benchmark"
			case "catch2-benchmark":
				wrapName = "catch2"
			}
			if wrapName != "" {
				if err := downloadMesonWrap(projectName, wrapName); err != nil {
					fmt.Printf("%sWarning: could not download %s wrap: %v%s\n", Yellow, wrapName, err, Reset)
				}
			}
		}
	} else {
		// Generate CMakeLists.txt (vcpkg or none)
		cmakeLists := templates.GenerateVcpkgCMakeLists(projectName, cppStandard, !cfg.IsLibrary, cfg.TestFramework != "" && cfg.TestFramework != "none", cfg.Benchmark, benchSources != nil, projectVersion)
		if err := os.WriteFile(filepath.Join(projectName, "CMakeLists.txt"), []byte(cmakeLists), 0644); err != nil {
			return fmt.Errorf("failed to write CMakeLists.txt: %w", err)
		}

		// Generate CMakePresets.json for vcpkg
		if cfg.PackageManager == "" || cfg.PackageManager == "vcpkg" {
			cmakePresets := templates.GenerateCMakePresets()
			if err := os.WriteFile(filepath.Join(projectName, "CMakePresets.json"), []byte(cmakePresets), 0644); err != nil {
				return fmt.Errorf("failed to write CMakePresets.json: %w", err)
			}
		}
	}

	// Generate version.hpp
	versionHpp := templates.GenerateVersionHpp(projectName, projectVersion)
	if err := os.WriteFile(filepath.Join(projectName, "include/"+projectName+"/version.hpp"), []byte(versionHpp), 0644); err != nil {
		return fmt.Errorf("failed to write version.hpp: %w", err)
	}

	// Generate header file
	libHeader := templates.GenerateLibHeader(projectName)
	if err := os.WriteFile(filepath.Join(projectName, "include/"+projectName+"/"+projectName+".hpp"), []byte(libHeader), 0644); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Generate main.cpp for executables
	if !cfg.IsLibrary {
		mainCpp := templates.GenerateMainCpp(projectName)
		if err := os.WriteFile(filepath.Join(projectName, "src/main.cpp"), []byte(mainCpp), 0644); err != nil {
			return fmt.Errorf("failed to write main.cpp: %w", err)
		}
	}

	// Generate library source file
	libSource := templates.GenerateLibSource(projectName)
	if err := os.WriteFile(filepath.Join(projectName, "src/"+projectName+".cpp"), []byte(libSource), 0644); err != nil {
		return fmt.Errorf("failed to write source: %w", err)
	}

	// Generate benchmark files if enabled
	if benchSources != nil {
		benchPath := filepath.Join(projectName, "bench", "bench_main.cpp")
		if err := os.WriteFile(benchPath, []byte(benchSources.Main), 0644); err != nil {
			return fmt.Errorf("failed to write bench_main.cpp: %w", err)
		}

		if cfg.PackageManager == "bazel" {
			// Generate bench/BUILD.bazel for Bazel projects
			benchBuild := templates.GenerateBuildBazelBench(projectName, cfg.Benchmark)
			if err := os.WriteFile(filepath.Join(projectName, "bench/BUILD.bazel"), []byte(benchBuild), 0644); err != nil {
				return fmt.Errorf("failed to write bench/BUILD.bazel: %w", err)
			}
		} else if cfg.PackageManager == "meson" {
			// Generate bench/meson.build for Meson projects
			benchMeson := templates.GenerateMesonBuildBench(projectName, cfg.Benchmark)
			if err := os.WriteFile(filepath.Join(projectName, "bench/meson.build"), []byte(benchMeson), 0644); err != nil {
				return fmt.Errorf("failed to write bench/meson.build: %w", err)
			}
		} else {
			// Generate bench/CMakeLists.txt for CMake projects
			benchCMake := templates.GenerateBenchCMake(projectName, cfg.Benchmark)
			if err := os.WriteFile(filepath.Join(projectName, "bench/CMakeLists.txt"), []byte(benchCMake), 0644); err != nil {
				return fmt.Errorf("failed to write bench/CMakeLists.txt: %w", err)
			}
		}
	}

	// Generate README based on package manager
	var readme string
	switch cfg.PackageManager {
	case "bazel":
		readme = templates.GenerateBazelReadme(projectName, cppStandard, cfg.IsLibrary)
	case "meson":
		readme = templates.GenerateMesonReadme(projectName, cppStandard, cfg.IsLibrary)
	default:
		readme = templates.GenerateVcpkgReadme(projectName, cppStandard, cfg.IsLibrary)
	}
	if err := os.WriteFile(filepath.Join(projectName, "README.md"), []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	// Generate .gitignore only if VCS is git
	if cfg.VCS == "" || cfg.VCS == "git" {
		var gitignore string
		if cfg.PackageManager == "bazel" {
			gitignore = templates.GenerateBazelGitignore()
		} else if cfg.PackageManager == "meson" {
			gitignore = templates.GenerateMesonGitignore()
		} else {
			gitignore = templates.GenerateGitignore()
		}
		if err := os.WriteFile(filepath.Join(projectName, ".gitignore"), []byte(gitignore), 0644); err != nil {
			return fmt.Errorf("failed to write .gitignore: %w", err)
		}
	}

	// Generate .clang-format
	clangFormatStyle := cfg.ClangFormat
	if clangFormatStyle == "" {
		clangFormatStyle = "Google"
	}
	clangFormat := templates.GenerateClangFormat(clangFormatStyle)
	if err := os.WriteFile(filepath.Join(projectName, ".clang-format"), []byte(clangFormat), 0644); err != nil {
		return fmt.Errorf("failed to write .clang-format: %w", err)
	}

	// Generate test files if test framework is selected
	if cfg.TestFramework != "" && cfg.TestFramework != "none" {
		if cfg.PackageManager == "bazel" {
			// Generate tests/BUILD.bazel for Bazel projects
			testsBuild := templates.GenerateBuildBazelTests(projectName, cfg.TestFramework)
			if err := os.WriteFile(filepath.Join(projectName, "tests/BUILD.bazel"), []byte(testsBuild), 0644); err != nil {
				return fmt.Errorf("failed to write tests/BUILD.bazel: %w", err)
			}
		} else if cfg.PackageManager == "meson" {
			// Generate tests/meson.build for Meson projects
			testsMeson := templates.GenerateMesonBuildTests(projectName, cfg.TestFramework)
			if err := os.WriteFile(filepath.Join(projectName, "tests/meson.build"), []byte(testsMeson), 0644); err != nil {
				return fmt.Errorf("failed to write tests/meson.build: %w", err)
			}
		} else {
			// Generate tests/CMakeLists.txt for CMake projects
			testCMake := templates.GenerateTestCMake(projectName, cfg.TestFramework)
			if err := os.WriteFile(filepath.Join(projectName, "tests/CMakeLists.txt"), []byte(testCMake), 0644); err != nil {
				return fmt.Errorf("failed to write tests/CMakeLists.txt: %w", err)
			}
		}

		testMain := templates.GenerateTestMain(projectName, cfg.TestFramework)
		if err := os.WriteFile(filepath.Join(projectName, "tests/test_main.cpp"), []byte(testMain), 0644); err != nil {
			return fmt.Errorf("failed to write tests/test_main.cpp: %w", err)
		}
	}

	// Generate cpx-ci.yaml file
	cpxCI := templates.GenerateCpxCI()
	if err := os.WriteFile(filepath.Join(projectName, "cpx-ci.yaml"), []byte(cpxCI), 0644); err != nil {
		return fmt.Errorf("failed to write cpx-ci.yaml: %w", err)
	}

	// Setup vcpkg if enabled (skip for bazel)
	if cfg.PackageManager == "vcpkg" {
		if vcpkgClient != nil {
			vcpkgPath, err := vcpkgClient.GetPath()
			if err == nil && vcpkgPath != "" {
				_ = setupVcpkgProject(vcpkgClient, projectName, projectName, cfg.IsLibrary, []string{})
			}
		}
	}

	// Skip CMake-based test/bench generation for Bazel projects
	// Bazel uses BUILD.bazel files in each directory instead

	// Initialize git and install hooks if configured
	if cfg.VCS == "git" || cfg.VCS == "" {
		// Initialize git repository
		gitInitCmd := exec.Command("git", "init")
		gitInitCmd.Dir = projectName
		if err := gitInitCmd.Run(); err == nil {
			// Install hooks if configured
			if cfg.UseHooks && (len(cfg.PreCommit) > 0 || len(cfg.PrePush) > 0) {
				// Change to project directory to install hooks
				originalDir, _ := os.Getwd()
				_ = os.Chdir(projectName)
				if err := git.InstallHooksWithConfig(cfg.PreCommit, cfg.PrePush); err != nil {
					// Non-fatal: just skip hooks if installation fails
					fmt.Printf("%sWarning: Could not install git hooks: %v%s\n", Yellow, err, Reset)
				}
				_ = os.Chdir(originalDir)
			}
		}
	}

	// Show success message
	fmt.Printf("\n%s✓ Project '%s' created successfully!%s\n\n", Green, projectName, Reset)
	fmt.Printf("  cd %s && cpx build && cpx run\n\n", projectName)

	return nil
}

// downloadMesonWrap installs a wrap file using 'meson wrap install'
func downloadMesonWrap(projectName, wrapName string) error {
	// Ensure meson is available
	if _, err := execLookPath("meson"); err != nil {
		return fmt.Errorf("meson not found in PATH: %w", err)
	}

	// We need to run this command inside the project directory
	cmd := execCommand("meson", "wrap", "install", wrapName)
	cmd.Dir = projectName
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("meson wrap install failed for %s: %w", wrapName, err)
	}

	fmt.Printf("  Installed %s.wrap\n", wrapName)
	return nil
}

func setupVcpkgProject(client *vcpkg.Client, targetDir, _ string, _ bool, dependencies []string) error {
	vcpkgPath, err := client.GetPath()
	if err != nil {
		return fmt.Errorf("vcpkg not configured: %w\n   Run: cpx config set-vcpkg-root <path>", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(targetDir); err != nil {
		return fmt.Errorf("failed to change to project directory: %w", err)
	}

	vcpkgCmd := exec.Command(vcpkgPath, "new", "--application")
	vcpkgCmd.Stdout = os.Stdout
	vcpkgCmd.Stderr = os.Stderr
	vcpkgCmd.Env = os.Environ()
	for i, env := range vcpkgCmd.Env {
		if strings.HasPrefix(env, "VCPKG_ROOT=") {
			vcpkgCmd.Env = append(vcpkgCmd.Env[:i], vcpkgCmd.Env[i+1:]...)
			break
		}
	}
	if err := vcpkgCmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize vcpkg.json: %w", err)
	}

	if len(dependencies) > 0 {
		fmt.Printf("%s Adding dependencies from template...%s\n", Cyan, Reset)
		for _, dep := range dependencies {
			if dep == "" {
				continue
			}
			fmt.Printf("   Adding %s...\n", dep)
			// vcpkg add requires "port" or "artifact" as the second argument
			// We're adding ports (packages), so use "port"
			addCmd := exec.Command(vcpkgPath, "add", "port", dep)
			addCmd.Stdout = os.Stdout
			addCmd.Stderr = os.Stderr
			addCmd.Env = vcpkgCmd.Env // Use same environment
			if err := addCmd.Run(); err != nil {
				fmt.Printf("%s  Warning: Failed to add dependency '%s': %v%s\n", Yellow, dep, err, Reset)
				// Continue with other dependencies even if one fails
			}
		}
	}

	return nil
}
