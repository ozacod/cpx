package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ozacod/cpx/internal/app/cli/tui"
	"github.com/ozacod/cpx/internal/pkg/templates"
)

// generateVcpkgProjectFilesFromConfig generates CMake files with vcpkg integration from config struct.
func generateVcpkgProjectFilesFromConfig(targetDir string, cfg *tui.ProjectConfig, projectName string, isLib bool) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	cppStandard := cfg.CppStandard
	if cppStandard == 0 {
		cppStandard = 17
	}

	projectVersion := "0.1.0"

	dependencies, err := getDependenciesFromVcpkgJsonLocal(targetDir)
	if err != nil {
		dependencies = []string{}
	}

	dirs := []string{
		"include/" + projectName,
		"src",
		"tests",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(targetDir, dir), 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	cmakeLists := templates.GenerateVcpkgCMakeLists(projectName, cppStandard, dependencies, !isLib, cfg.TestFramework != "" && cfg.TestFramework != "none", cfg.TestFramework, projectVersion)
	if err := os.WriteFile(filepath.Join(targetDir, "CMakeLists.txt"), []byte(cmakeLists), 0644); err != nil {
		return fmt.Errorf("failed to write CMakeLists.txt: %w", err)
	}

	if cfg.PackageManager == "" || cfg.PackageManager == "vcpkg" {
		cmakePresets := templates.GenerateCMakePresets()
		if err := os.WriteFile(filepath.Join(targetDir, "CMakePresets.json"), []byte(cmakePresets), 0644); err != nil {
			return fmt.Errorf("failed to write CMakePresets.json: %w", err)
		}
	}

	versionHpp := templates.GenerateVersionHpp(projectName, projectVersion)
	if err := os.WriteFile(filepath.Join(targetDir, "include/"+projectName+"/version.hpp"), []byte(versionHpp), 0644); err != nil {
		return fmt.Errorf("failed to write version.hpp: %w", err)
	}

	libHeader := templates.GenerateLibHeader(projectName)
	if err := os.WriteFile(filepath.Join(targetDir, "include/"+projectName+"/"+projectName+".hpp"), []byte(libHeader), 0644); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	if !isLib {
		mainCpp := templates.GenerateMainCpp(projectName, dependencies)
		if err := os.WriteFile(filepath.Join(targetDir, "src/main.cpp"), []byte(mainCpp), 0644); err != nil {
			return fmt.Errorf("failed to write main.cpp: %w", err)
		}
	}

	libSource := templates.GenerateLibSource(projectName, dependencies)
	if err := os.WriteFile(filepath.Join(targetDir, "src/"+projectName+".cpp"), []byte(libSource), 0644); err != nil {
		return fmt.Errorf("failed to write source: %w", err)
	}

	readme := templates.GenerateVcpkgReadme(projectName, dependencies, cppStandard, isLib)
	if err := os.WriteFile(filepath.Join(targetDir, "README.md"), []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	if cfg.VCS == "" || cfg.VCS == "git" {
		gitignore := templates.GenerateGitignore()
		if err := os.WriteFile(filepath.Join(targetDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
			return fmt.Errorf("failed to write .gitignore: %w", err)
		}
	}

	clangFormatStyle := cfg.ClangFormat
	if clangFormatStyle == "" {
		clangFormatStyle = "Google"
	}
	clangFormat := templates.GenerateClangFormat(clangFormatStyle)
	if err := os.WriteFile(filepath.Join(targetDir, ".clang-format"), []byte(clangFormat), 0644); err != nil {
		return fmt.Errorf("failed to write .clang-format: %w", err)
	}

	if cfg.TestFramework != "" && cfg.TestFramework != "none" {
		testCMake := templates.GenerateTestCMake(projectName, dependencies, cfg.TestFramework)
		if err := os.WriteFile(filepath.Join(targetDir, "tests/CMakeLists.txt"), []byte(testCMake), 0644); err != nil {
			return fmt.Errorf("failed to write tests/CMakeLists.txt: %w", err)
		}

		testMain := templates.GenerateTestMain(projectName, dependencies, cfg.TestFramework)
		if err := os.WriteFile(filepath.Join(targetDir, "tests/test_main.cpp"), []byte(testMain), 0644); err != nil {
			return fmt.Errorf("failed to write tests/test_main.cpp: %w", err)
		}
	}

	cpxCI := templates.GenerateCpxCI()
	if err := os.WriteFile(filepath.Join(targetDir, "cpx.ci"), []byte(cpxCI), 0644); err != nil {
		return fmt.Errorf("failed to write cpx.ci: %w", err)
	}

	return nil
}

func getDependenciesFromVcpkgJsonLocal(projectDir string) ([]string, error) {
	vcpkgJsonPath := filepath.Join(projectDir, "vcpkg.json")
	if _, err := os.Stat(vcpkgJsonPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	data, err := os.ReadFile(vcpkgJsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read vcpkg.json: %w", err)
	}

	var vcpkgJson map[string]interface{}
	if err := json.Unmarshal(data, &vcpkgJson); err != nil {
		return nil, fmt.Errorf("failed to parse vcpkg.json: %w", err)
	}

	deps, ok := vcpkgJson["dependencies"].([]interface{})
	if !ok {
		return []string{}, nil
	}

	dependencies := make([]string, 0, len(deps))
	for _, dep := range deps {
		if depStr, ok := dep.(string); ok {
			dependencies = append(dependencies, depStr)
		}
	}

	return dependencies, nil
}
