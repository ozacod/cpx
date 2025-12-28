package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ToolchainConfig represents the cpx-ci.yaml structure
// Simplified structure:
// - runners: execution environments (docker/ssh) with optional compiler settings
// - toolchains: named build configurations referencing a runner
type ToolchainConfig struct {
	Runners    []Runner    `yaml:"runners,omitempty"`
	Toolchains []Toolchain `yaml:"toolchains,omitempty"`
}

type Runner struct {
	Name  string `yaml:"name"`
	Type  string `yaml:"type,omitempty"`  // docker, ssh (native/local if omitted)
	Image string `yaml:"image,omitempty"` // for docker
	Host  string `yaml:"host,omitempty"`  // for ssh
	User  string `yaml:"user,omitempty"`  // for ssh
	// Compiler settings (optional, can be set in runner)
	CC                 string `yaml:"cc,omitempty"`
	CXX                string `yaml:"cxx,omitempty"`
	CMakeToolchainFile string `yaml:"cmake_toolchain_file,omitempty"`
}

func (r *Runner) IsNative() bool {
	return r.Type == "" || r.Type == "native" || r.Type == "local"
}

func (r *Runner) IsDocker() bool {
	return r.Type == "docker"
}

func (r *Runner) IsSSH() bool {
	return r.Type == "ssh"
}

// Toolchain defines a build configuration (renamed from BuildConfig)
type Toolchain struct {
	Name         string            `yaml:"name"`
	Runner       string            `yaml:"runner,omitempty"` // references Runner.Name
	Active       *bool             `yaml:"active,omitempty"` // true (default) or false to disable
	BuildType    string            `yaml:"build_type,omitempty"`
	CMakeOptions []string          `yaml:"cmake_options,omitempty"`
	BuildOptions []string          `yaml:"build_options,omitempty"`
	Env          map[string]string `yaml:"env,omitempty"`
	Optimization string            `yaml:"optimization,omitempty"` // "0", "1", "2", "3", "s", "fast"
	Jobs         int               `yaml:"jobs,omitempty"`         // number of parallel jobs
}

func (t *Toolchain) IsActive() bool {
	if t.Active == nil {
		return true
	}
	return *t.Active
}

func LoadToolchains(path string) (*ToolchainConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config ToolchainConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cpx-ci.yaml: %w", err)
	}

	// Set defaults for each toolchain
	for i := range config.Toolchains {
		if config.Toolchains[i].BuildType == "" {
			config.Toolchains[i].BuildType = "Release"
		}
	}

	return &config, nil
}

func (c *ToolchainConfig) FindRunner(name string) *Runner {
	for i := range c.Runners {
		if c.Runners[i].Name == name {
			return &c.Runners[i]
		}
	}
	return nil
}

func (c *ToolchainConfig) FindToolchain(name string) *Toolchain {
	for i := range c.Toolchains {
		if c.Toolchains[i].Name == name {
			return &c.Toolchains[i]
		}
	}
	return nil
}

func (c *ToolchainConfig) GetOutputDir() string {
	return filepath.Join(".bin", "ci")
}

func SaveToolchains(config *ToolchainConfig, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal cpx-ci.yaml: %w", err)
	}

	header := "# cpx-ci.yaml - CI toolchain configuration\n# runners: execution environments (docker/ssh) with optional compiler settings\n# toolchains: named build configurations\n\n"
	content := header + string(data)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write cpx-ci.yaml: %w", err)
	}

	return nil
}
