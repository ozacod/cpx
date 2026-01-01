package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ozacod/cpx/internal/build/bazel"
	"github.com/ozacod/cpx/internal/build/cmake"
	"github.com/ozacod/cpx/internal/build/common"
	build "github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/build/meson"
	"github.com/ozacod/cpx/internal/build/vcpkg"
	"github.com/ozacod/cpx/internal/cli/tui"
	cpxconfig "github.com/ozacod/cpx/internal/config"
	"github.com/ozacod/cpx/internal/templates"
	"github.com/ozacod/cpx/internal/templates/project_templates"
	"github.com/ozacod/cpx/internal/utils/colors"
	"github.com/ozacod/cpx/internal/utils/git"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new C++ project (interactive)",
		Long:  "Create a new C++ project using an interactive TUI. This will guide you through the project configuration.",
		Example: `  cpx new            # launch the interactive creator
  cpx new --help    # view options`,
		RunE: runNew,
		Args: cobra.NoArgs,
	}

	return cmd
}

func runNew(_ *cobra.Command, _ []string) error {
	// Initialize and run the TUI
	p := tea.NewProgram(tui.InitialModel())
	m, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	finalModel, ok := m.(tui.Model)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	if finalModel.IsCancelled() {
		return nil
	}

	config := finalModel.GetConfig()

	return createProjectFromTUI(config)
}

func createProjectFromTUI(config tui.ProjectConfig) error {
	projectName := config.Name

	if _, err := os.Stat(projectName); err == nil {
		return fmt.Errorf("directory '%s' already exists", projectName)
	}

	if config.UseTemplate {
		template, ok := project_templates.GetTemplateByName(config.TemplateName)
		if !ok {
			return fmt.Errorf("template '%s' not found", config.TemplateName)
		}

		cppStandard := config.CppStandard
		if cppStandard == 0 {
			cppStandard = cpxconfig.DefaultCppStandard
		}

		templateConfig := project_templates.TemplateConfig{
			ProjectName:    projectName,
			PackageManager: config.PackageManager,
			CppStandard:    cppStandard,
		}

		return template.Generate(templateConfig)
	}

	// Custom project creation flow
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
		cfg.VCS = cpxconfig.DefaultVCS
	}

	// Set PackageManager configuration defaults
	if cfg.PackageManager == "" {
		cfg.PackageManager = cpxconfig.DefaultPackageManager
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
		cppStandard = cpxconfig.DefaultCppStandard
	}

	projectVersion := cpxconfig.DefaultVersion

	benchSources, _ := templates.GenerateBenchmarkSources(projectName, cfg.Benchmark)

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

	// Initialize the builder based on package manager
	var builder build.BuildSystem
	switch cfg.PackageManager {
	case "bazel":
		builder = bazel.New()
	case "meson":
		builder = meson.New()
	case "cmake":
		builder = cmake.New()
	default:
		builder = vcpkg.New()
	}

	initConfig := build.InitConfig{
		Name:          projectName,
		Version:       projectVersion,
		IsLibrary:     cfg.IsLibrary,
		CppStandard:   cppStandard,
		TestFramework: cfg.TestFramework,
		Benchmark:     cfg.Benchmark,
	}

	if err := builder.GenerateBuildSrc(projectName, initConfig); err != nil {
		return fmt.Errorf("failed to generate build source files: %w", err)
	}

	versionHpp := templates.GenerateVersionHpp(projectName, projectVersion)
	if err := os.WriteFile(filepath.Join(projectName, "include/"+projectName+"/version.hpp"), []byte(versionHpp), 0644); err != nil {
		return fmt.Errorf("failed to write version.hpp: %w", err)
	}

	libHeader := templates.GenerateLibHeader(projectName)
	if err := os.WriteFile(filepath.Join(projectName, "include/"+projectName+"/"+projectName+".hpp"), []byte(libHeader), 0644); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if !cfg.IsLibrary {
		mainCpp := templates.GenerateMainCpp(projectName)
		if err := os.WriteFile(filepath.Join(projectName, "src/main.cpp"), []byte(mainCpp), 0644); err != nil {
			return fmt.Errorf("failed to write main.cpp: %w", err)
		}
	}

	libSource := templates.GenerateLibSource(projectName)
	if err := os.WriteFile(filepath.Join(projectName, "src/"+projectName+".cpp"), []byte(libSource), 0644); err != nil {
		return fmt.Errorf("failed to write source: %w", err)
	}

	if benchSources != nil {
		benchPath := filepath.Join(projectName, "bench", "bench_main.cpp")
		if err := os.WriteFile(benchPath, []byte(benchSources.Main), 0644); err != nil {
			return fmt.Errorf("failed to write bench_main.cpp: %w", err)
		}

		if err := builder.GenerateBuildBench(projectName, initConfig); err != nil {
			return fmt.Errorf("failed to generate benchmark build files: %w", err)
		}
	}

	readme := builder.GenerateReadme(initConfig)
	if err := os.WriteFile(filepath.Join(projectName, "README.md"), []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	if cfg.VCS == "" || cfg.VCS == "git" {
		if err := builder.GenerateGitignore(projectName); err != nil {
			return fmt.Errorf("failed to generate .gitignore: %w", err)
		}
	}

	clangFormatStyle := cfg.ClangFormat
	if clangFormatStyle == "" {
		clangFormatStyle = cpxconfig.DefaultClangFormat
	}
	clangFormat := templates.GenerateClangFormat(clangFormatStyle)
	if err := os.WriteFile(filepath.Join(projectName, common.ClangFormatFile), []byte(clangFormat), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.ClangFormatFile, err)
	}

	if cfg.TestFramework != "" && cfg.TestFramework != "none" {
		if err := builder.GenerateBuildTest(projectName, initConfig); err != nil {
			return fmt.Errorf("failed to generate test build files: %w", err)
		}

		testMain := templates.GenerateTestMain(projectName, cfg.TestFramework)
		if err := os.WriteFile(filepath.Join(projectName, "tests/test_main.cpp"), []byte(testMain), 0644); err != nil {
			return fmt.Errorf("failed to write tests/test_main.cpp: %w", err)
		}
	}

	cpxCI := templates.GenerateCpxCI()
	if err := os.WriteFile(filepath.Join(projectName, "cpx-ci.yaml"), []byte(cpxCI), 0644); err != nil {
		return fmt.Errorf("failed to write cpx-ci.yaml: %w", err)
	}

	// Setup vcpkg if enabled (skip for bazel)
	if cfg.PackageManager == "vcpkg" {
		// Use the existing builder if it's a vcpkg builder, or create a new one to query path (though we should just cast if possible)
		vcpkgBuilder, ok := builder.(*vcpkg.VcpkgBuilder)
		if ok {
			vcpkgPath, err := vcpkgBuilder.GetPath()
			if err == nil && vcpkgPath != "" {
				// Initialize vcpkg project structure
				_ = setupVcpkgProject(vcpkgBuilder, projectName, []string{})
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
					fmt.Printf("%sWarning: Could not install git hooks: %v%s\n", colors.Yellow, err, colors.Reset)
				}
				_ = os.Chdir(originalDir)
			}
		}
	}

	fmt.Printf("\n%s✓ Project '%s' created successfully!%s\n\n", colors.Green, projectName, colors.Reset)
	fmt.Printf("  cd %s && cpx build && cpx run\n\n", projectName)

	return nil
}

func setupVcpkgProject(builder *vcpkg.VcpkgBuilder, targetDir string, dependencies []string) error {
	vcpkgPath, err := builder.GetPath()
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
		fmt.Printf("%s Adding dependencies from template...%s\n", colors.Cyan, colors.Reset)
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
				fmt.Printf("%s  Warning: Failed to add dependency '%s': %v%s\n", colors.Yellow, dep, err, colors.Reset)
				// Continue with other dependencies even if one fails
			}
		}
	}

	return nil
}
