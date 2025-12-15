package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozacod/cpx/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveTargetConfig(t *testing.T) {
	tests := []struct {
		name             string
		targetName       string
		expectedName     string
		expectedRunner   string
		expectedMode     string
		expectedImage    string
		expectedPlatform string
	}{
		{
			name:             "Linux AMD64",
			targetName:       "linux-amd64",
			expectedName:     "linux-amd64",
			expectedRunner:   "docker",
			expectedMode:     "build",
			expectedImage:    "cpx-linux-amd64",
			expectedPlatform: "linux/amd64",
		},
		{
			name:             "Linux ARM64",
			targetName:       "linux-arm64",
			expectedName:     "linux-arm64",
			expectedRunner:   "docker",
			expectedMode:     "build",
			expectedImage:    "cpx-linux-arm64",
			expectedPlatform: "linux/arm64",
		},
		{
			name:             "Linux AMD64 MUSL",
			targetName:       "linux-amd64-musl",
			expectedName:     "linux-amd64-musl",
			expectedRunner:   "docker",
			expectedMode:     "build",
			expectedImage:    "cpx-linux-amd64-musl",
			expectedPlatform: "linux/amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveTargetConfig(tt.targetName)
			assert.Equal(t, tt.expectedName, result.Name)
			assert.Equal(t, tt.expectedRunner, result.Runner)
			require.NotNil(t, result.Docker)
			assert.Equal(t, tt.expectedMode, result.Docker.Mode)
			assert.Equal(t, tt.expectedImage, result.Docker.Image)
			assert.Equal(t, tt.expectedPlatform, result.Docker.Platform)
			require.NotNil(t, result.Docker.Build)
			assert.Contains(t, result.Docker.Build.Dockerfile, "Dockerfile."+tt.targetName)
		})
	}
}

func TestSaveCIConfig(t *testing.T) {
	tmpDir := t.TempDir()
	ciPath := filepath.Join(tmpDir, "cpx-ci.yaml")

	// Create test config with new format
	ciConfig := &config.CIConfig{
		Targets: []config.CITarget{
			{
				Name:   "linux-amd64",
				Runner: "docker",
				Docker: &config.DockerConfig{
					Mode:     "pull",
					Image:    "ubuntu:22.04",
					Platform: "linux/amd64",
				},
			},
		},
		Build: config.CIBuild{
			Type:         "Release",
			Optimization: "2",
			Jobs:         0,
		},
		Output: ".bin/ci",
	}

	// Save config
	err := config.SaveCI(ciConfig, ciPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(ciPath)
	require.NoError(t, err)

	// Load it back
	loadedConfig, err := config.LoadCI(ciPath)
	require.NoError(t, err)

	// Verify content
	assert.Len(t, loadedConfig.Targets, 1)
	assert.Equal(t, "linux-amd64", loadedConfig.Targets[0].Name)
	assert.Equal(t, "docker", loadedConfig.Targets[0].Runner)
	require.NotNil(t, loadedConfig.Targets[0].Docker)
	assert.Equal(t, "pull", loadedConfig.Targets[0].Docker.Mode)
	assert.Equal(t, "ubuntu:22.04", loadedConfig.Targets[0].Docker.Image)
	assert.Equal(t, "Release", loadedConfig.Build.Type)
	assert.Equal(t, ".bin/ci", loadedConfig.Output)
}

// TestRunAddTargetWithArgs removed because add-target now uses interactive TUI
// which cannot be tested without a TTY

func TestRunRemoveTarget(t *testing.T) {
	// Setup: create temp dir
	tmpDir := t.TempDir()

	// Change to temp dir for cpx-ci.yaml I/O
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create initial cpx-ci.yaml with 3 targets using new format
	ciConfig := &config.CIConfig{
		Targets: []config.CITarget{
			{Name: "linux-amd64", Runner: "docker", Docker: &config.DockerConfig{Mode: "pull", Image: "ubuntu:22.04"}},
			{Name: "linux-arm64", Runner: "docker", Docker: &config.DockerConfig{Mode: "pull", Image: "ubuntu:22.04"}},
			{Name: "windows-amd64", Runner: "docker", Docker: &config.DockerConfig{Mode: "pull", Image: "ubuntu:22.04"}},
		},
		Build:  config.CIBuild{Type: "Release", Optimization: "2", Jobs: 0},
		Output: ".bin/ci",
	}
	require.NoError(t, config.SaveCI(ciConfig, "cpx-ci.yaml"))

	// Test 1: Remove single target
	err := runRemoveTarget(nil, []string{"linux-amd64"})
	require.NoError(t, err)

	// Verify
	loaded, err := config.LoadCI("cpx-ci.yaml")
	require.NoError(t, err)
	require.Len(t, loaded.Targets, 2)
	assert.Equal(t, "linux-arm64", loaded.Targets[0].Name)
	assert.Equal(t, "windows-amd64", loaded.Targets[1].Name)

	// Test 2: Remove multiple targets
	err = runRemoveTarget(nil, []string{"linux-arm64", "windows-amd64"})
	require.NoError(t, err)

	// Verify
	loaded, err = config.LoadCI("cpx-ci.yaml")
	require.NoError(t, err)
	require.Len(t, loaded.Targets, 0)

	// Test 3: Remove non-existent target (should warn but succeed for valid ones, or fail if none match)
	// Reset config
	ciConfig.Targets = []config.CITarget{{Name: "target1", Runner: "docker", Docker: &config.DockerConfig{Mode: "pull", Image: "ubuntu:22.04"}}}
	require.NoError(t, config.SaveCI(ciConfig, "cpx-ci.yaml"))

	// If none match, it should return nil (based on implementation) but print message
	err = runRemoveTarget(nil, []string{"non-existent"})
	require.NoError(t, err) // Should not return error, just print "No matching targets"

	loaded, err = config.LoadCI("cpx-ci.yaml")
	require.NoError(t, err)
	assert.Len(t, loaded.Targets, 1) // Should remain unchanged
}
