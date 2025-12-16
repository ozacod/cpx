package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozacod/cpx/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveToolchainConfig(t *testing.T) {
	tmpDir := t.TempDir()
	ciPath := filepath.Join(tmpDir, "cpx-ci.yaml")

	// Create test config with new format
	ciConfig := &config.ToolchainConfig{
		Toolchains: []config.Toolchain{
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
		Build: config.ToolchainBuild{
			Type:         "Release",
			Optimization: "2",
			Jobs:         0,
		},
		Output: ".bin/ci",
	}

	// Save config
	err := config.SaveToolchains(ciConfig, ciPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(ciPath)
	require.NoError(t, err)

	// Load it back
	loadedConfig, err := config.LoadToolchains(ciPath)
	require.NoError(t, err)

	// Verify content
	assert.Len(t, loadedConfig.Toolchains, 1)
	assert.Equal(t, "linux-amd64", loadedConfig.Toolchains[0].Name)
	assert.Equal(t, "docker", loadedConfig.Toolchains[0].Runner)
	require.NotNil(t, loadedConfig.Toolchains[0].Docker)
	assert.Equal(t, "pull", loadedConfig.Toolchains[0].Docker.Mode)
	assert.Equal(t, "ubuntu:22.04", loadedConfig.Toolchains[0].Docker.Image)
	assert.Equal(t, "Release", loadedConfig.Build.Type)
	assert.Equal(t, ".bin/ci", loadedConfig.Output)
}

func TestRunRemoveToolchain(t *testing.T) {
	// Setup: create temp dir
	tmpDir := t.TempDir()

	// Change to temp dir for cpx-ci.yaml I/O
	oldWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(oldWd) }()

	// Create initial cpx-ci.yaml with 3 toolchains using new format
	ciConfig := &config.ToolchainConfig{
		Toolchains: []config.Toolchain{
			{Name: "linux-amd64", Runner: "docker", Docker: &config.DockerConfig{Mode: "pull", Image: "ubuntu:22.04"}},
			{Name: "linux-arm64", Runner: "docker", Docker: &config.DockerConfig{Mode: "pull", Image: "ubuntu:22.04"}},
			{Name: "windows-amd64", Runner: "docker", Docker: &config.DockerConfig{Mode: "pull", Image: "ubuntu:22.04"}},
		},
		Build:  config.ToolchainBuild{Type: "Release", Optimization: "2", Jobs: 0},
		Output: ".bin/ci",
	}
	require.NoError(t, config.SaveToolchains(ciConfig, "cpx-ci.yaml"))

	// Test 1: Remove single toolchain
	err := runRemoveToolchainCmd(nil, []string{"linux-amd64"})
	require.NoError(t, err)

	// Verify
	loaded, err := config.LoadToolchains("cpx-ci.yaml")
	require.NoError(t, err)
	require.Len(t, loaded.Toolchains, 2)
	assert.Equal(t, "linux-arm64", loaded.Toolchains[0].Name)
	assert.Equal(t, "windows-amd64", loaded.Toolchains[1].Name)

	// Test 2: Remove multiple toolchains
	err = runRemoveToolchainCmd(nil, []string{"linux-arm64", "windows-amd64"})
	require.NoError(t, err)

	// Verify
	loaded, err = config.LoadToolchains("cpx-ci.yaml")
	require.NoError(t, err)
	require.Len(t, loaded.Toolchains, 0)

	// Test 3: Remove non-existent toolchain (should warn but succeed for valid ones, or fail if none match)
	// Reset config
	ciConfig.Toolchains = []config.Toolchain{{Name: "target1", Runner: "docker", Docker: &config.DockerConfig{Mode: "pull", Image: "ubuntu:22.04"}}}
	require.NoError(t, config.SaveToolchains(ciConfig, "cpx-ci.yaml"))

	// If none match, it should return nil (based on implementation) but print message
	err = runRemoveToolchainCmd(nil, []string{"non-existent"})
	require.NoError(t, err) // Should not return error, just print "No matching toolchains"

	loaded, err = config.LoadToolchains("cpx-ci.yaml")
	require.NoError(t, err)
	assert.Len(t, loaded.Toolchains, 1) // Should remain unchanged
}
