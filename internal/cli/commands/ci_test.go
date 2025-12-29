package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozacod/cpx/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveToolchainConfig(t *testing.T) {
	tmpDir := t.TempDir()
	ciPath := filepath.Join(tmpDir, "cpx-ci.yaml")

	// Create test config with simplified structure
	ciConfig := &config.ToolchainConfig{
		Runners: []config.Runner{
			{Name: "ubuntu-docker", Type: "docker", Image: "ubuntu:22.04"},
		},
		Toolchains: []config.Toolchain{
			{Name: "linux-release", Runner: "ubuntu-docker", BuildType: "Release"},
		},
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

	// Verify runners
	assert.Len(t, loadedConfig.Runners, 1)
	assert.Equal(t, "ubuntu-docker", loadedConfig.Runners[0].Name)
	assert.Equal(t, "docker", loadedConfig.Runners[0].Type)

	// Verify toolchains (build configs)
	assert.Len(t, loadedConfig.Toolchains, 1)
	assert.Equal(t, "linux-release", loadedConfig.Toolchains[0].Name)
	assert.Equal(t, "ubuntu-docker", loadedConfig.Toolchains[0].Runner)
	assert.Equal(t, "Release", loadedConfig.Toolchains[0].BuildType)

	// Output is always .bin/ci
	assert.Equal(t, filepath.Join(".bin", "ci"), loadedConfig.GetOutputDir())
}

func TestFindToolchainAndRunner(t *testing.T) {
	ciConfig := &config.ToolchainConfig{
		Runners: []config.Runner{
			{Name: "docker-ubuntu", Type: "docker", Image: "ubuntu:22.04"},
			{Name: "local"},
		},
		Toolchains: []config.Toolchain{
			{Name: "release-build", Runner: "docker-ubuntu", BuildType: "Release"},
			{Name: "debug-build", Runner: "local", BuildType: "Debug"},
		},
	}

	// Test FindToolchain
	tc := ciConfig.FindToolchain("release-build")
	require.NotNil(t, tc)
	assert.Equal(t, "docker-ubuntu", tc.Runner)

	tc = ciConfig.FindToolchain("nonexistent")
	assert.Nil(t, tc)

	// Test FindRunner
	r := ciConfig.FindRunner("docker-ubuntu")
	require.NotNil(t, r)
	assert.Equal(t, "docker", r.Type)

	r = ciConfig.FindRunner("local")
	require.NotNil(t, r)
	assert.True(t, r.IsNative())

	r = ciConfig.FindRunner("nonexistent")
	assert.Nil(t, r)
}

func TestToolchainDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	ciPath := filepath.Join(tmpDir, "cpx-ci.yaml")

	content := `
toolchains:
  - name: default-test
    runner: ubuntu
`
	err := os.WriteFile(ciPath, []byte(content), 0644)
	require.NoError(t, err)

	ciConfig, err := config.LoadToolchains(ciPath)
	require.NoError(t, err)

	require.Len(t, ciConfig.Toolchains, 1)
	tc := ciConfig.Toolchains[0]

	// Verify defaults
	assert.Equal(t, "Release", tc.BuildType)
	assert.Equal(t, "", tc.Optimization) // Empty by default, ci.go handles the O2 default
	assert.Equal(t, 0, tc.Jobs)
	assert.True(t, tc.IsActive())
}

func TestToolchainIsActive(t *testing.T) {
	active := true
	inactive := false

	tcActive := config.Toolchain{Name: "active", Active: &active}
	tcInactive := config.Toolchain{Name: "inactive", Active: &inactive}
	tcDefault := config.Toolchain{Name: "default"}

	assert.True(t, tcActive.IsActive())
	assert.False(t, tcInactive.IsActive())
	assert.True(t, tcDefault.IsActive())
}

func TestRunnerTypes(t *testing.T) {
	dockerRunner := config.Runner{Name: "docker-test", Type: "docker", Image: "ubuntu:22.04"}
	assert.True(t, dockerRunner.IsDocker())
	assert.False(t, dockerRunner.IsNative())
	assert.False(t, dockerRunner.IsSSH())

	nativeRunner := config.Runner{Name: "local"}
	assert.True(t, nativeRunner.IsNative())
	assert.False(t, nativeRunner.IsDocker())

	nativeRunnerExplicit := config.Runner{Name: "local", Type: "native"}
	assert.True(t, nativeRunnerExplicit.IsNative())

	localRunner := config.Runner{Name: "local", Type: "local"}
	assert.True(t, localRunner.IsNative())

	sshRunner := config.Runner{Name: "build-server", Type: "ssh", Host: "server.local"}
	assert.True(t, sshRunner.IsSSH())
	assert.False(t, sshRunner.IsNative())
	assert.False(t, sshRunner.IsDocker())
}
