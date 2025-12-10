package config_test

import (
	"os"
	"testing"

	"github.com/ozacod/cpx/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadGlobalConfig(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func() error
		cleanupFunc  func() error
		expectsError bool
	}{
		{
			name: "Valid config file",
			setupFunc: func() error {
				configDir, err := config.GetConfigDir()
				if err != nil {
					return err
				}
				configFile, err := config.GetConfigPath()
				if err != nil {
					return err
				}

				// Create config directory
				if err := os.MkdirAll(configDir, 0755); err != nil {
					return err
				}

				// Create a valid config file
				configContent := `bcr_root: /tmp/test_bcr
vcpkg_root: /tmp/test_vcpkg
`
				return os.WriteFile(configFile, []byte(configContent), 0644)
			},
			cleanupFunc: func() error {
				configFile, err := config.GetConfigPath()
				if err != nil {
					return err
				}
				return os.Remove(configFile)
			},
			expectsError: false,
		},
		{
			name: "Invalid config file",
			setupFunc: func() error {
				configDir, err := config.GetConfigDir()
				if err != nil {
					return err
				}
				configFile, err := config.GetConfigPath()
				if err != nil {
					return err
				}

				// Create config directory
				if err := os.MkdirAll(configDir, 0755); err != nil {
					return err
				}

				// Create an invalid config file
				configContent := `invalid: yaml: content: [
`
				return os.WriteFile(configFile, []byte(configContent), 0644)
			},
			cleanupFunc: func() error {
				configFile, err := config.GetConfigPath()
				if err != nil {
					return err
				}
				return os.Remove(configFile)
			},
			expectsError: true,
		},
		{
			name: "Missing config file",
			setupFunc: func() error {
				// No setup needed - file should not exist
				return nil
			},
			cleanupFunc: func() error {
				// No cleanup needed
				return nil
			},
			expectsError: false, // Should return default config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			if tt.setupFunc != nil {
				defer tt.cleanupFunc()
				require.NoError(t, tt.setupFunc())
			}

			cfg, err := config.LoadGlobal()

			if tt.expectsError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				// For missing config file, we expect default values (which may be empty)
				// Just verify we got a valid config object
			}
		})
	}
}

func TestSaveGlobalConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       *config.GlobalConfig
		expectsError bool
	}{
		{
			name: "Valid config",
			config: &config.GlobalConfig{
				BcrRoot:   "/test/bcr",
				VcpkgRoot: "/test/vcpkg",
			},
			expectsError: false,
		},
		{
			name: "Empty config",
			config: &config.GlobalConfig{
				BcrRoot:   "",
				VcpkgRoot: "",
			},
			expectsError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up after test
			defer func() {
				configFile, err := config.GetConfigPath()
				if err != nil {
					return
				}
				if _, err := os.Stat(configFile); err == nil {
					os.Remove(configFile)
				}
			}()

			err := config.SaveGlobal(tt.config)

			if tt.expectsError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify we can load the saved config
				loadedConfig, err := config.LoadGlobal()
				assert.NoError(t, err)
				assert.NotNil(t, loadedConfig)
				assert.Equal(t, tt.config.BcrRoot, loadedConfig.BcrRoot)
				assert.Equal(t, tt.config.VcpkgRoot, loadedConfig.VcpkgRoot)
			}
		})
	}
}
