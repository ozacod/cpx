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
		name               string
		targetName         string
		expectedDockerfile string
		// expectedPlatform   string
	}{
		{
			name:               "Linux AMD64",
			targetName:         "linux-amd64",
			expectedDockerfile: "linux-amd64",
			// expectedPlatform:   "linux/amd64",
		},
		{
			name:               "Linux ARM64",
			targetName:         "linux-arm64",
			expectedDockerfile: "linux-arm64",
			// expectedPlatform:   "linux/arm64",
		},
		{
			name:               "Linux AMD64 MUSL",
			targetName:         "linux-amd64-musl",
			expectedDockerfile: "linux-amd64-musl",
			// expectedPlatform:   "linux/amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveTargetConfig(tt.targetName)
			// deriveTargetConfig only sets Source and Platform
			// Name and Tag are derived by LoadCI when loading the config
			assert.Equal(t, tt.expectedDockerfile, result.Source)
			// assert.Equal(t, tt.expectedPlatform, result.Platform)
			assert.Empty(t, result.Name) // Not set by deriveTargetConfig
			assert.Empty(t, result.Tag)  // Not set by deriveTargetConfig
		})
	}
}

func TestSaveCIConfig(t *testing.T) {
	tmpDir := t.TempDir()
	ciPath := filepath.Join(tmpDir, "cpx.ci")

	// Create test config
	ciConfig := &config.CIConfig{
		Targets: []config.CITarget{
			{
				Name:    "linux-amd64",
				Source:  "linux-amd64",
				Tag:     "cpx-linux-amd64",
				Triplet: "x64-linux",
				// Platform:   "linux/amd64",
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
	assert.Equal(t, "Release", loadedConfig.Build.Type)
	assert.Equal(t, ".bin/ci", loadedConfig.Output)
}

func TestRunAddTargetWithArgs(t *testing.T) {
	// Setup: create temp dir with mock dockerfiles
	tmpDir := t.TempDir()
	dockerfilesDir := filepath.Join(tmpDir, ".config", "cpx", "dockerfiles")
	require.NoError(t, os.MkdirAll(dockerfilesDir, 0755))

	// Create mock Dockerfiles
	require.NoError(t, os.WriteFile(filepath.Join(dockerfilesDir, "Dockerfile.linux-arm64"), []byte("FROM ubuntu"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dockerfilesDir, "Dockerfile.linux-amd64"), []byte("FROM ubuntu"), 0644))

	// Change HOME to temp dir for test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Change to temp dir for cpx.ci output
	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0755))
	oldWd, _ := os.Getwd()
	os.Chdir(projectDir)
	defer os.Chdir(oldWd)

	// Test: add linux-arm64 target via args
	err := runAddTarget(nil, []string{"linux-arm64"})
	require.NoError(t, err)

	// Verify cpx.ci was created with correct target
	ciConfig, err := config.LoadCI("cpx.ci")
	require.NoError(t, err)
	require.Len(t, ciConfig.Targets, 1)
	assert.Equal(t, "linux-arm64", ciConfig.Targets[0].Name)
	// assert.Equal(t, "linux/arm64", ciConfig.Targets[0].Platform)

	// Test: add another target
	err = runAddTarget(nil, []string{"linux-amd64"})
	require.NoError(t, err)

	// Verify both targets exist
	ciConfig, err = config.LoadCI("cpx.ci")
	require.NoError(t, err)
	require.Len(t, ciConfig.Targets, 2)

	// Test: adding duplicate should skip
	err = runAddTarget(nil, []string{"linux-arm64"})
	require.NoError(t, err)

	// Should still have 2 targets (not 3)
	ciConfig, err = config.LoadCI("cpx.ci")
	require.NoError(t, err)
	assert.Len(t, ciConfig.Targets, 2)

	// Test: invalid target should error
	err = runAddTarget(nil, []string{"invalid-target"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown target")
}

func TestRunRemoveTarget(t *testing.T) {
	// Setup: create temp dir
	tmpDir := t.TempDir()

	// Change to temp dir for cpx.ci I/O
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create initial cpx.ci with 2 targets
	ciConfig := &config.CIConfig{
		Targets: []config.CITarget{
			{Name: "linux-amd64", Source: "linux-amd64"},
			{Name: "linux-arm64", Source: "linux-arm64"},
			{Name: "windows-amd64", Source: "windows-amd64"},
		},
		Build:  config.CIBuild{Type: "Release", Optimization: "2", Jobs: 0},
		Output: ".bin/ci",
	}
	require.NoError(t, config.SaveCI(ciConfig, "cpx.ci"))

	// Test 1: Remove single target
	err := runRemoveTarget(nil, []string{"linux-amd64"})
	require.NoError(t, err)

	// Verify
	loaded, err := config.LoadCI("cpx.ci")
	require.NoError(t, err)
	require.Len(t, loaded.Targets, 2)
	assert.Equal(t, "linux-arm64", loaded.Targets[0].Name)
	assert.Equal(t, "windows-amd64", loaded.Targets[1].Name)

	// Test 2: Remove multiple targets
	err = runRemoveTarget(nil, []string{"linux-arm64", "windows-amd64"})
	require.NoError(t, err)

	// Verify
	loaded, err = config.LoadCI("cpx.ci")
	require.NoError(t, err)
	require.Len(t, loaded.Targets, 0)

	// Test 3: Remove non-existent target (should warn but succeed for valid ones, or fail if none match)
	// Reset config
	ciConfig.Targets = []config.CITarget{{Name: "target1", Source: "target1"}}
	require.NoError(t, config.SaveCI(ciConfig, "cpx.ci"))

	// If none match, it should return nil (based on implementation) but print message
	err = runRemoveTarget(nil, []string{"non-existent"})
	require.NoError(t, err) // Should not return error, just print "No matching targets"

	loaded, err = config.LoadCI("cpx.ci")
	require.NoError(t, err)
	assert.Len(t, loaded.Targets, 1) // Should remain unchanged
}
