package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// CIConfig represents the cpx.ci structure for cross-compilation
type CIConfig struct {
	Targets []CITarget `yaml:"targets"`
	Build   CIBuild    `yaml:"build"`
	Output  string     `yaml:"output"`
}

// CITarget represents a cross-compilation target
type CITarget struct {
	Name       string `yaml:"name"`
	Dockerfile string `yaml:"dockerfile"`
	Image      string `yaml:"image"`
	Triplet    string `yaml:"triplet"`
	Platform   string `yaml:"platform"`
}

// CIBuild represents CI build configuration
type CIBuild struct {
	Type         string   `yaml:"type"`
	Optimization string   `yaml:"optimization"`
	Jobs         int      `yaml:"jobs"`
	CMakeArgs    []string `yaml:"cmake_args"`
	BuildArgs    []string `yaml:"build_args"`
	MesonArgs    []string `yaml:"meson_args"`
}

// LoadCI loads the CI configuration from cpx.ci
func LoadCI(path string) (*CIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config CIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cpx.ci: %w", err)
	}

	// Set defaults
	if config.Output == "" {
		config.Output = "out"
	}
	if config.Build.Type == "" {
		config.Build.Type = "Release"
	}
	if config.Build.Optimization == "" {
		config.Build.Optimization = "2"
	}

	return &config, nil
}
