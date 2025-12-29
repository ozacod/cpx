package project_templates

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ozacod/cpx/internal/pkg/build/bazel"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/internal/pkg/build/meson"
	"github.com/ozacod/cpx/internal/pkg/build/vcpkg"
	"github.com/ozacod/cpx/internal/pkg/templates"
	"github.com/ozacod/cpx/internal/pkg/utils/colors"
)

// TemplateConfig holds the configuration for template generation
type TemplateConfig struct {
	ProjectName    string
	PackageManager string // "vcpkg", "meson", or "bazel"
	CppStandard    int
}

// ProjectTemplate defines the interface for all project templates
type ProjectTemplate interface {
	// Name returns the template name (used for selection)
	Name() string
	// Description returns a short description of the template
	Description() string
	// Generate creates all files for this template
	Generate(config TemplateConfig) error
	// Dependencies returns the list of package dependencies
	Dependencies() []string
}

// TemplateInfo contains display information for a template
type TemplateInfo struct {
	Name        string
	Description string
	Template    ProjectTemplate
}

// Registry holds all available templates
var Registry []TemplateInfo

func RegisterTemplate(t ProjectTemplate) {
	Registry = append(Registry, TemplateInfo{
		Name:        t.Name(),
		Description: t.Description(),
		Template:    t,
	})
}

func GetTemplateNames() []string {
	names := make([]string, len(Registry))
	for i, t := range Registry {
		names[i] = t.Name
	}
	return names
}

func GetTemplateByName(name string) (ProjectTemplate, bool) {
	for _, t := range Registry {
		if strings.EqualFold(t.Name, name) {
			return t.Template, true
		}
	}
	return nil, false
}

// BaseTemplateHelper provides common utilities for templates
type BaseTemplateHelper struct{}

func (h *BaseTemplateHelper) CreateProjectStructure(projectName string, dirs []string) error {
	// Create project root
	if err := os.MkdirAll(projectName, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Create subdirectories
	for _, dir := range dirs {
		dirPath := filepath.Join(projectName, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", dirPath, err)
		}
	}
	return nil
}

func (h *BaseTemplateHelper) WriteFile(projectName, relativePath, content string) error {
	fullPath := filepath.Join(projectName, relativePath)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for '%s': %w", relativePath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write '%s': %w", relativePath, err)
	}
	return nil
}

func (h *BaseTemplateHelper) GetBuilder(packageManager string) build.BuildSystem {
	switch packageManager {
	case "bazel":
		return bazel.New()
	case "meson":
		return meson.New()
	default:
		return vcpkg.New()
	}
}

func (h *BaseTemplateHelper) GenerateCommonFiles(config TemplateConfig) error {
	projectName := config.ProjectName

	// Generate .clang-format
	clangFormat := templates.GenerateClangFormat("Google")
	if err := h.WriteFile(projectName, ".clang-format", clangFormat); err != nil {
		return err
	}

	// Generate cpx-ci.yaml
	cpxCI := templates.GenerateCpxCI()
	if err := h.WriteFile(projectName, "cpx-ci.yaml", cpxCI); err != nil {
		return err
	}

	// Generate .gitignore based on build system
	builder := h.GetBuilder(config.PackageManager)
	if err := builder.GenerateGitignore(projectName); err != nil {
		return fmt.Errorf("failed to generate .gitignore: %w", err)
	}

	return nil
}

func (h *BaseTemplateHelper) InitGitRepo(projectName string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = projectName
	return cmd.Run()
}

func (h *BaseTemplateHelper) SetupVcpkg(projectName string, dependencies []string) error {
	vcpkgBuilder := vcpkg.New()
	vcpkgPath, err := vcpkgBuilder.GetPath()
	if err != nil || vcpkgPath == "" {
		// vcpkg not configured, skip
		return nil
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(projectName); err != nil {
		return err
	}

	// Initialize vcpkg.json
	vcpkgCmd := exec.Command(vcpkgPath, "new", "--application")
	vcpkgCmd.Stdout = os.Stdout
	vcpkgCmd.Stderr = os.Stderr
	// Remove VCPKG_ROOT from environment
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "VCPKG_ROOT=") {
			env = append(env[:i], env[i+1:]...)
			break
		}
	}
	vcpkgCmd.Env = env
	if err := vcpkgCmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize vcpkg.json: %w", err)
	}

	// Add dependencies
	if len(dependencies) > 0 {
		fmt.Printf("%s Adding dependencies...%s\n", colors.Cyan, colors.Reset)
		for _, dep := range dependencies {
			if dep == "" {
				continue
			}
			fmt.Printf("   Adding %s...\n", dep)
			addCmd := exec.Command(vcpkgPath, "add", "port", dep)
			addCmd.Stdout = os.Stdout
			addCmd.Stderr = os.Stderr
			addCmd.Env = env
			if err := addCmd.Run(); err != nil {
				fmt.Printf("%s  Warning: Failed to add dependency '%s': %v%s\n", colors.Yellow, dep, err, colors.Reset)
			}
		}
	}

	return nil
}

func (h *BaseTemplateHelper) PrintSuccess(projectName string) {
	fmt.Printf("\n%s✓ Project '%s' created successfully!%s\n\n", colors.Green, projectName, colors.Reset)
	fmt.Printf("  cd %s && cpx build && cpx run\n\n", projectName)
}
