package bazel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	builder := New()
	projectName := "test-project"

	// Create project directory
	require.NoError(t, os.MkdirAll(projectName, 0755))
	projectPath := filepath.Join(tmpDir, projectName)

	initConfig := build.InitConfig{
		Name:          projectName,
		Version:       "0.1.0",
		IsLibrary:     false,
		CppStandard:   20,
		TestFramework: "googletest",
		Benchmark:     "google-benchmark",
	}

	// Test GenerateGitignore
	err = builder.GenerateGitignore(projectPath)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(projectPath, ".gitignore"))

	// Test GenerateBuildSrc
	err = builder.GenerateBuildSrc(projectPath, initConfig)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(projectPath, "MODULE.bazel"))
	assert.FileExists(t, filepath.Join(projectPath, "BUILD.bazel"))
	assert.FileExists(t, filepath.Join(projectPath, "src", "BUILD.bazel"))
	assert.FileExists(t, filepath.Join(projectPath, "include", "BUILD.bazel"))
	assert.FileExists(t, filepath.Join(projectPath, ".bazelrc"))
	assert.FileExists(t, filepath.Join(projectPath, ".bazelignore"))

	// Test GenerateBuildTest
	err = builder.GenerateBuildTest(projectPath, initConfig)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(projectPath, "tests", "BUILD.bazel"))

	// Test GenerateBuildBench
	err = builder.GenerateBuildBench(projectPath, initConfig)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(projectPath, "bench", "BUILD.bazel"))
}
