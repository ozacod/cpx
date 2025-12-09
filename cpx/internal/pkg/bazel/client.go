package bazel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Client handles Bazel Central Registry operations
type Client struct {
	registryPath string
}

// Module represents a BCR module
type Module struct {
	Name        string   `json:"name"`
	Versions    []string `json:"versions"`
	Homepage    string   `json:"homepage"`
	Maintainers []string `json:"maintainers"`
}

// Dependency represents a bazel_dep entry in MODULE.bazel
type Dependency struct {
	Name    string
	Version string
}

// ModuleMetadata represents the metadata.json structure in BCR
type ModuleMetadata struct {
	Homepage    string `json:"homepage"`
	Maintainers []struct {
		Email  string `json:"email"`
		GitHub string `json:"github"`
		Name   string `json:"name"`
	} `json:"maintainers"`
	Versions       []string          `json:"versions"`
	YankedVersions map[string]string `json:"yanked_versions"`
}

// NewClient creates a BCR client with the given registry path
func NewClient(registryPath string) *Client {
	return &Client{
		registryPath: registryPath,
	}
}

// GetModulesDir returns the path to the modules directory
func (c *Client) GetModulesDir() string {
	return filepath.Join(c.registryPath, "modules")
}

// ListModules returns all available module names
func (c *Client) ListModules() ([]string, error) {
	modulesDir := c.GetModulesDir()
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read modules directory: %w", err)
	}

	var modules []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			modules = append(modules, entry.Name())
		}
	}
	return modules, nil
}

// SearchModules searches for modules by name pattern
func (c *Client) SearchModules(query string) ([]Module, error) {
	allModules, err := c.ListModules()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []Module

	for _, name := range allModules {
		if strings.Contains(strings.ToLower(name), query) {
			module, err := c.GetModule(name)
			if err != nil {
				continue // Skip modules with metadata errors
			}
			results = append(results, *module)
		}
	}

	return results, nil
}

// GetModule fetches metadata for a specific module
func (c *Client) GetModule(moduleName string) (*Module, error) {
	metadataPath := filepath.Join(c.GetModulesDir(), moduleName, "metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("module %s not found: %w", moduleName, err)
	}

	var metadata ModuleMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata for %s: %w", moduleName, err)
	}

	module := &Module{
		Name:     moduleName,
		Versions: metadata.Versions,
		Homepage: metadata.Homepage,
	}

	for _, m := range metadata.Maintainers {
		if m.Name != "" {
			module.Maintainers = append(module.Maintainers, m.Name)
		} else if m.GitHub != "" {
			module.Maintainers = append(module.Maintainers, m.GitHub)
		}
	}

	return module, nil
}

// GetLatestVersion returns the latest version of a module
func (c *Client) GetLatestVersion(moduleName string) (string, error) {
	module, err := c.GetModule(moduleName)
	if err != nil {
		return "", err
	}

	if len(module.Versions) == 0 {
		return "", fmt.Errorf("no versions available for %s", moduleName)
	}

	// Versions are typically listed in order, last is latest
	return module.Versions[len(module.Versions)-1], nil
}

// GetModuleVersions fetches available versions for a module
func (c *Client) GetModuleVersions(moduleName string) ([]string, error) {
	module, err := c.GetModule(moduleName)
	if err != nil {
		return nil, err
	}
	return module.Versions, nil
}

// AddDependency adds a bazel_dep to MODULE.bazel
func AddDependency(modulePath, depName, version string) error {
	content, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	// Check if dependency already exists
	depPattern := regexp.MustCompile(fmt.Sprintf(`bazel_dep\s*\(\s*name\s*=\s*"%s"`, regexp.QuoteMeta(depName)))
	if depPattern.Match(content) {
		// Update existing dependency
		updatePattern := regexp.MustCompile(fmt.Sprintf(`(bazel_dep\s*\(\s*name\s*=\s*"%s"\s*,\s*version\s*=\s*")[^"]*(")\)`, regexp.QuoteMeta(depName)))
		newContent := updatePattern.ReplaceAll(content, []byte(fmt.Sprintf(`${1}%s${2})`, version)))
		return os.WriteFile(modulePath, newContent, 0644)
	}

	// Add new dependency at the end
	newDep := fmt.Sprintf("\nbazel_dep(name = \"%s\", version = \"%s\")\n", depName, version)
	content = append(content, []byte(newDep)...)
	return os.WriteFile(modulePath, content, 0644)
}

// ListDependencies returns current bazel_dep entries from MODULE.bazel
func ListDependencies(modulePath string) ([]Dependency, error) {
	content, err := os.ReadFile(modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	// Match bazel_dep(name = "xxx", version = "yyy")
	pattern := regexp.MustCompile(`bazel_dep\s*\(\s*name\s*=\s*"([^"]+)"\s*,\s*version\s*=\s*"([^"]+)"\s*\)`)
	matches := pattern.FindAllStringSubmatch(string(content), -1)

	var deps []Dependency
	for _, match := range matches {
		if len(match) >= 3 {
			deps = append(deps, Dependency{
				Name:    match[1],
				Version: match[2],
			})
		}
	}

	return deps, nil
}

// RemoveDependency removes a bazel_dep from MODULE.bazel
func RemoveDependency(modulePath, depName string) error {
	content, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("failed to read MODULE.bazel: %w", err)
	}

	// Remove the dependency line
	pattern := regexp.MustCompile(fmt.Sprintf(`\n?bazel_dep\s*\(\s*name\s*=\s*"%s"[^)]*\)\n?`, regexp.QuoteMeta(depName)))
	newContent := pattern.ReplaceAll(content, []byte(""))

	return os.WriteFile(modulePath, newContent, 0644)
}

// SortedModuleNames returns module names sorted alphabetically
func SortedModuleNames(modules []Module) []string {
	names := make([]string, len(modules))
	for i, m := range modules {
		names[i] = m.Name
	}
	sort.Strings(names)
	return names
}
