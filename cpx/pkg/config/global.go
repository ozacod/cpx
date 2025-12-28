package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	VcpkgRoot  string `yaml:"vcpkg_root"`
	BcrRoot    string `yaml:"bcr_root"`    // Bazel Central Registry path
	WrapdbRoot string `yaml:"wrapdb_root"` // Meson WrapDB path
}

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

func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

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

func SaveGlobal(config *GlobalConfig) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

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
