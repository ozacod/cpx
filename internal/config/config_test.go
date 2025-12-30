package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozacod/cpx/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadGlobalConfig(t *testing.T) {
	tests := []struct {
		name         string
		configYAML   string
		expectsError bool
		hasFile      bool
	}{
		{
			name: "Valid config file",
			configYAML: `bcr_root: /tmp/test_bcr
vcpkg_root: /tmp/test_vcpkg
`,
			expectsError: false,
			hasFile:      true,
		},
		{
			name:         "Invalid config file",
			configYAML:   `invalid: yaml: content: [`,
			expectsError: true,
			hasFile:      true,
		},
		{
			name:         "Missing config file",
			configYAML:   "",
			expectsError: false, // Should return default config
			hasFile:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for isolation
			tmpDir := t.TempDir()

			// Set HOME to temp dir so config goes there
			oldHome := os.Getenv("HOME")
			defer func() { assert.NoError(t, os.Setenv("HOME", oldHome)) }()
			require.NoError(t, os.Setenv("HOME", tmpDir))

			if tt.hasFile {
				configDir := filepath.Join(tmpDir, ".config", "cpx")
				require.NoError(t, os.MkdirAll(configDir, 0755))
				configFile := filepath.Join(configDir, "config.yaml")
				require.NoError(t, os.WriteFile(configFile, []byte(tt.configYAML), 0644))
			}

			cfg, err := config.LoadGlobal()

			if tt.expectsError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
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
			// Use temp directory for isolation
			tmpDir := t.TempDir()

			// Set HOME to temp dir so config goes there
			oldHome := os.Getenv("HOME")
			defer func() { assert.NoError(t, os.Setenv("HOME", oldHome)) }()
			require.NoError(t, os.Setenv("HOME", tmpDir))

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
