package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// GlobalConfig represents the global cpx configuration
type GlobalConfig struct {
	VcpkgRoot string `yaml:"vcpkg_root"`
	BcrRoot   string `yaml:"bcr_root"` // Bazel Central Registry path
}

// GetConfigDir returns the directory where cpx stores its global config
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Use ~/.config/cpx on Unix, %APPDATA%/cpx on Windows
	var configDir string
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			// Fallback to AppData\Roaming in user's home directory if APPDATA is not set
			configDir = filepath.Join(homeDir, "AppData", "Roaming", "cpx")
		} else {
			configDir = filepath.Join(appData, "cpx")
		}
	} else {
		configDir = filepath.Join(homeDir, ".config", "cpx")
	}

	return configDir, nil
}

// GetConfigPath returns the path to the global cpx config file
func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

// LoadGlobal loads the global cpx configuration
// If the config file doesn't exist, it will be created with default values
func LoadGlobal() (*GlobalConfig, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// If config doesn't exist, create it with default values
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := &GlobalConfig{}
		if err := SaveGlobal(defaultConfig); err != nil {
			return nil, fmt.Errorf("failed to create config file: %w", err)
		}
		return defaultConfig, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// SaveGlobal saves the global cpx configuration
func SaveGlobal(config *GlobalConfig) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
