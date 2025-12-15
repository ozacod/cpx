package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ozacod/cpx/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigShow(t *testing.T) {
	tests := []struct {
		name         string
		config       *config.GlobalConfig
		expectsError bool
	}{
		{
			name: "Show config with values",
			config: &config.GlobalConfig{
				VcpkgRoot:  "/test/vcpkg",
				BcrRoot:    "/test/bcr",
				WrapdbRoot: "/test/wrapdb",
			},
			expectsError: false,
		},
		{
			name:         "Show config with empty values",
			config:       &config.GlobalConfig{},
			expectsError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for isolation
			tmpDir := t.TempDir()
			oldHome := os.Getenv("HOME")
			defer os.Setenv("HOME", oldHome)
			os.Setenv("HOME", tmpDir)

			// Save test config
			require.NoError(t, config.SaveGlobal(tt.config))

			// Capture output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			runErr := showConfig()

			// Restore stdout
			if err := w.Close(); err != nil {
				t.Fatalf("Failed to close pipe: %v", err)
			}
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				t.Fatalf("Failed to read from pipe: %v", err)
			}
			output := buf.String()

			if tt.expectsError {
				assert.Error(t, runErr)
			} else {
				assert.NoError(t, runErr)
				assert.Contains(t, output, "Cpx Configuration")
				assert.Contains(t, output, "vcpkg_root:")
				assert.Contains(t, output, "bcr_root:")
				assert.Contains(t, output, "wrapdb_root:")
			}
		})
	}
}

func TestConfigGet(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		config       *config.GlobalConfig
		expected     string
		expectsError bool
	}{
		{
			name: "Get vcpkg_root",
			key:  "vcpkg_root",
			config: &config.GlobalConfig{
				VcpkgRoot: "/test/vcpkg",
			},
			expected:     "/test/vcpkg",
			expectsError: false,
		},
		{
			name: "Get bcr_root",
			key:  "bcr-root",
			config: &config.GlobalConfig{
				BcrRoot: "/test/bcr",
			},
			expected:     "/test/bcr",
			expectsError: false,
		},
		{
			name:         "Get unknown key",
			key:          "unknown_key",
			config:       &config.GlobalConfig{},
			expectsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for isolation
			tmpDir := t.TempDir()
			oldHome := os.Getenv("HOME")
			defer os.Setenv("HOME", oldHome)
			os.Setenv("HOME", tmpDir)

			// Save test config
			require.NoError(t, config.SaveGlobal(tt.config))

			// Capture output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			runErr := getConfig(tt.key)

			// Restore stdout
			if err := w.Close(); err != nil {
				t.Fatalf("Failed to close pipe: %v", err)
			}
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				t.Fatalf("Failed to read from pipe: %v", err)
			}
			output := buf.String()

			if tt.expectsError {
				assert.Error(t, runErr)
			} else {
				assert.NoError(t, runErr)
				assert.Contains(t, output, tt.expected)
			}
		})
	}
}

func TestSetVcpkgRoot(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectsError bool
		createPath   bool
	}{
		{
			name:         "Set valid vcpkg root",
			path:         "/test/vcpkg",
			expectsError: false,
			createPath:   true,
		},
		{
			name:         "Set invalid vcpkg root (path doesn't exist)",
			path:         "/nonexistent/vcpkg",
			expectsError: true,
			createPath:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for isolation
			tmpDir := t.TempDir()
			oldHome := os.Getenv("HOME")
			defer os.Setenv("HOME", oldHome)
			os.Setenv("HOME", tmpDir)

			if tt.createPath {
				// Create the path
				testPath := filepath.Join(tmpDir, "vcpkg")
				require.NoError(t, os.MkdirAll(testPath, 0755))
				// Create vcpkg executable
				require.NoError(t, os.WriteFile(filepath.Join(testPath, "vcpkg"), []byte("#"), 0755))

				// Update path to use temp directory
				tt.path = testPath
			}

			// Capture output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := setVcpkgRoot(tt.path)

			// Restore stdout
			w.Close()
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)

			if tt.expectsError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify config was saved
				cfg, err := config.LoadGlobal()
				require.NoError(t, err)
				assert.Equal(t, tt.path, cfg.VcpkgRoot)
			}
		})
	}
}

func TestSetBcrRoot(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectsError bool
		createPath   bool
	}{
		{
			name:         "Set valid BCR root",
			path:         "/test/bcr",
			expectsError: false,
			createPath:   true,
		},
		{
			name:         "Set invalid BCR root (path doesn't exist)",
			path:         "/nonexistent/bcr",
			expectsError: true,
			createPath:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for isolation
			tmpDir := t.TempDir()
			oldHome := os.Getenv("HOME")
			defer os.Setenv("HOME", oldHome)
			os.Setenv("HOME", tmpDir)

			if tt.createPath {
				// Create the path
				testPath := filepath.Join(tmpDir, "bcr")
				require.NoError(t, os.MkdirAll(testPath, 0755))
				// Create modules directory
				require.NoError(t, os.MkdirAll(filepath.Join(testPath, "modules"), 0755))

				// Update path to use temp directory
				tt.path = testPath
			}

			// Capture output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			runErr := setBcrRoot(tt.path)

			// Restore stdout
			w.Close()
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			_, err := buf.ReadFrom(r)
			require.NoError(t, err)

			if tt.expectsError {
				assert.Error(t, runErr)
			} else {
				assert.NoError(t, runErr)

				// Verify config was saved
				cfg, err := config.LoadGlobal()
				require.NoError(t, err)
				assert.Equal(t, tt.path, cfg.BcrRoot)
			}
		})
	}
}

func TestSetWrapdbRoot(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectsError bool
		createPath   bool
	}{
		{
			name:         "Set valid wrapdb root",
			path:         "/test/wrapdb",
			expectsError: false,
			createPath:   true,
		},
		{
			name:         "Set invalid wrapdb root (path doesn't exist)",
			path:         "/nonexistent/wrapdb",
			expectsError: true,
			createPath:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for isolation
			tmpDir := t.TempDir()
			oldHome := os.Getenv("HOME")
			defer func(key, value string) {
				err := os.Setenv(key, value)
				if err != nil {
					t.Fatalf("Failed to restore %s: %v", key, err)
				}
			}("HOME", oldHome)
			err := os.Setenv("HOME", tmpDir)
			if err != nil {
				t.Fatalf("Failed to set HOME: %v", err)
			}

			if tt.createPath {
				// Create the path
				testPath := filepath.Join(tmpDir, "wrapdb")
				require.NoError(t, os.MkdirAll(testPath, 0755))

				// Update path to use temp directory
				tt.path = testPath
			}

			// Capture output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			runErr := setWrapdbRoot(tt.path)

			// Restore stdout
			if err := w.Close(); err != nil {
				t.Fatalf("Failed to close pipe: %v", err)
			}
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				t.Fatalf("Failed to read from pipe: %v", err)
			}

			if tt.expectsError {
				assert.Error(t, runErr)
			} else {
				assert.NoError(t, runErr)

				// Verify config was saved
				cfg, err := config.LoadGlobal()
				require.NoError(t, err)
				assert.Equal(t, tt.path, cfg.WrapdbRoot)
			}
		})
	}
}
