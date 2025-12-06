package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ozacod/cpx/internal/app/cli/tui"
	"github.com/spf13/cobra"
)

var newGetVcpkgPathFunc func() (string, error)
var newSetupVcpkgProjectFunc func(string, string, bool, []string) error
var newGenerateVcpkgProjectFilesFromConfigFunc func(string, *CpxConfig, string, bool) error

// NewNewCmd creates the new command with interactive TUI
func NewNewCmd(getVcpkgPath func() (string, error), setupVcpkgProject func(string, string, bool, []string) error, generateVcpkgProjectFilesFromConfig func(string, *CpxConfig, string, bool) error) *cobra.Command {
	newGetVcpkgPathFunc = getVcpkgPath
	newSetupVcpkgProjectFunc = setupVcpkgProject
	newGenerateVcpkgProjectFilesFromConfigFunc = generateVcpkgProjectFilesFromConfig

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

func runNew(cmd *cobra.Command, args []string) error {
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
	return createProjectFromTUI(config, newGetVcpkgPathFunc, newSetupVcpkgProjectFunc, newGenerateVcpkgProjectFilesFromConfigFunc)
}

func createProjectFromTUI(config tui.ProjectConfig, getVcpkgPath func() (string, error), setupVcpkgProject func(string, string, bool, []string) error, generateVcpkgProjectFilesFromConfig func(string, *CpxConfig, string, bool) error) error {
	projectName := config.Name

	// Check if directory already exists
	if _, err := os.Stat(projectName); err == nil {
		return fmt.Errorf("directory '%s' already exists", projectName)
	}

	// Create the new directory
	if err := os.MkdirAll(projectName, 0755); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", projectName, err)
	}

	// Create configuration from TUI choices (no external templates needed)
	cfg := &CpxConfig{}
	cfg.Package.Name = projectName
	cfg.Package.Version = "0.1.0"
	cfg.Package.CppStandard = config.CppStandard
	cfg.Build.SharedLibs = config.IsLibrary
	cfg.Build.ClangFormat = config.ClangFormat
	cfg.Testing.Framework = config.TestFramework
	cfg.VCS.Type = config.VCS
	cfg.PackageManager.Type = config.PackageManager

	// Set hooks
	if len(config.GitHooks) > 0 {
		for _, hook := range config.GitHooks {
			if hook == "fmt" || hook == "lint" {
				cfg.Hooks.PreCommit = append(cfg.Hooks.PreCommit, hook)
			}
			if hook == "test" {
				cfg.Hooks.PrePush = append(cfg.Hooks.PrePush, hook)
			}
		}
	}

	// Set VCS configuration
	if config.VCS == "" {
		config.VCS = "git" // Default to git for backward compatibility
	}
	cfg.VCS.Type = config.VCS

	// Set PackageManager configuration
	if config.PackageManager == "" {
		config.PackageManager = "vcpkg" // Default to vcpkg for backward compatibility
	}
	cfg.PackageManager.Type = config.PackageManager

	// Initialize git repository only if VCS is set to git
	if config.VCS == "git" {
		cmd := exec.Command("git", "init")
		cmd.Dir = projectName
		_ = cmd.Run() // Ignore errors silently
	}

	// Create directory structure
	dirs := []string{"src", "include", "tests", "scripts", "docs"}
	for _, dir := range dirs {
		dirPath := filepath.Join(projectName, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", dirPath, err)
		}
	}

	// Create main source file
	mainFilePath := filepath.Join(projectName, "src", "main.cpp")
	mainContent := generateMainContent(projectName, config.IsLibrary)
	if err := os.WriteFile(mainFilePath, []byte(mainContent), 0644); err != nil {
		return fmt.Errorf("failed to write main.cpp: %w", err)
	}

	// Create header file if it's a library
	if config.IsLibrary {
		headerFilePath := filepath.Join(projectName, "include", projectName+".hpp")
		headerContent := generateHeaderContent(projectName)
		if err := os.WriteFile(headerFilePath, []byte(headerContent), 0644); err != nil {
			return fmt.Errorf("failed to write header file: %w", err)
		}
	}

	// Create test file if test framework is selected
	if config.TestFramework != "none" {
		testFilePath := filepath.Join(projectName, "tests", "test_main.cpp")
		testContent := generateTestContent(config.TestFramework)
		if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
			return fmt.Errorf("failed to write test file: %w", err)
		}
	}

	// Create README
	readmePath := filepath.Join(projectName, "README.md")
	readmeContent := generateReadmeContent(projectName, config)
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to write README.md: %w", err)
	}

	// Create .gitignore only if VCS is git
	if config.VCS == "git" {
		gitignorePath := filepath.Join(projectName, ".gitignore")
		gitignoreContent := generateGitignoreContent()
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			return fmt.Errorf("failed to write .gitignore: %w", err)
		}
	}

	// Always generate project files (CMakeLists.txt, etc.)
	if err := generateVcpkgProjectFilesFromConfig(projectName, cfg, projectName, config.IsLibrary); err != nil {
		return fmt.Errorf("failed to generate project files: %w", err)
	}

	// Setup vcpkg if enabled
	if config.PackageManager == "vcpkg" {
		vcpkgPath, err := getVcpkgPath()
		if err == nil && vcpkgPath != "" {
			_ = setupVcpkgProject(projectName, projectName, config.IsLibrary, []string{})
		}
	}

	// Show next steps only
	fmt.Printf("\n%sNext steps:%s\n", Cyan, Reset)
	fmt.Printf("  cd %s\n", projectName)
	fmt.Printf("  cpx build\n")
	fmt.Printf("  cpx run\n\n")

	return nil
}

func generateMainContent(projectName string, isLibrary bool) string {
	if isLibrary {
		return fmt.Sprintf(`#include "%s.hpp"

namespace %s {

void hello() {
    // TODO: Implement library functionality
}

} // namespace %s
`, projectName, projectName, projectName)
	}

	return `#include <iostream>

int main() {
    std::cout << "Hello, World!" << std::endl;
    return 0;
}
`
}

func generateHeaderContent(projectName string) string {
	guard := strings.ToUpper(projectName) + "_HPP"
	return fmt.Sprintf(`#ifndef %s
#define %s

namespace %s {

void hello();

} // namespace %s

#endif // %s
`, guard, guard, projectName, projectName, guard)
}

func generateTestContent(framework string) string {
	if framework == "catch2" {
		return `#define CATCH_CONFIG_MAIN
#include <catch2/catch.hpp>

TEST_CASE("Example test", "[example]") {
    REQUIRE(1 + 1 == 2);
}
`
	}

	// GoogleTest
	return `#include <gtest/gtest.h>

TEST(ExampleTest, BasicAssertion) {
    EXPECT_EQ(1 + 1, 2);
}

int main(int argc, char **argv) {
    ::testing::InitGoogleTest(&argc, argv);
    return RUN_ALL_TESTS();
}
`
}

func generateReadmeContent(projectName string, config tui.ProjectConfig) string {
	projectType := "executable"
	if config.IsLibrary {
		projectType = "library"
	}

	return fmt.Sprintf(`# %s

A C++ %s project.

## Features

- C++%d standard
- Test framework: %s
- Code formatting: %s

## Building

`+"```bash"+`
cpx build
`+"```"+`

## Running

`+"```bash"+`
cpx run
`+"```"+`

## Testing

`+"```bash"+`
cpx test
`+"```"+`

## License

MIT
`, projectName, projectType, config.CppStandard, config.TestFramework, config.ClangFormat)
}

func generateGitignoreContent() string {
	return `# Build directories
build/
bin/
lib/
*.out
*.exe

# IDE files
.vscode/
.idea/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db

# vcpkg
vcpkg_installed/
.vcpkg/

# Compiled Object files
*.o
*.obj

# Debug files
*.dSYM/
*.pdb
`
}
