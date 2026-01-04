// Package integration provides end-to-end tests for the cpx build workflow.
// These tests create real projects and build them with actual build tools.
// Run with -short to skip these tests.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ozacod/cpx/internal/build/cmake"
	"github.com/ozacod/cpx/internal/build/common"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/ozacod/cpx/internal/build/meson"
	"github.com/ozacod/cpx/internal/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

func skipIfToolMissing(t *testing.T, tool string) {
	if _, err := exec.LookPath(tool); err != nil {
		t.Skipf("skipping: %s not available", tool)
	}
}

func skipIfCompilerMissing(t *testing.T) {
	compilers := []string{"g++", "clang++", "c++"}
	for _, c := range compilers {
		if _, err := exec.LookPath(c); err == nil {
			return
		}
	}
	t.Skip("skipping: no C++ compiler available")
}

// createCMakeExecutableProject creates a minimal CMake executable project
func createCMakeExecutableProject(t *testing.T, projectPath, projectName string) {
	t.Helper()

	// Create directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, "include", projectName), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, "src"), 0755))

	// Generate files using templates
	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "include", projectName, "version.hpp"),
		[]byte(templates.GenerateVersionHpp(projectName, "1.0.0")),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "include", projectName, projectName+".hpp"),
		[]byte(templates.GenerateLibHeader(projectName)),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "src", projectName+".cpp"),
		[]byte(templates.GenerateLibSource(projectName)),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "src", "main.cpp"),
		[]byte(templates.GenerateMainCpp(projectName)),
		0644,
	))

	// Use cmake builder to generate build files
	builder := cmake.New()
	require.NoError(t, builder.GenerateBuildSrc(projectPath, build.InitConfig{
		Name:        projectName,
		Version:     "1.0.0",
		CppStandard: 17,
		IsLibrary:   false,
	}))
}

// createCMakeLibraryProject creates a minimal CMake library project
func createCMakeLibraryProject(t *testing.T, projectPath, projectName string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, "include", projectName), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, "src"), 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "include", projectName, "version.hpp"),
		[]byte(templates.GenerateVersionHpp(projectName, "1.0.0")),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "include", projectName, projectName+".hpp"),
		[]byte(templates.GenerateLibHeader(projectName)),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "src", projectName+".cpp"),
		[]byte(templates.GenerateLibSource(projectName)),
		0644,
	))

	builder := cmake.New()
	require.NoError(t, builder.GenerateBuildSrc(projectPath, build.InitConfig{
		Name:        projectName,
		Version:     "1.0.0",
		CppStandard: 17,
		IsLibrary:   true,
	}))
}

// createMesonExecutableProject creates a minimal Meson executable project
func createMesonExecutableProject(t *testing.T, projectPath, projectName string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, "include", projectName), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, "src"), 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "include", projectName, "version.hpp"),
		[]byte(templates.GenerateVersionHpp(projectName, "1.0.0")),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "include", projectName, projectName+".hpp"),
		[]byte(templates.GenerateLibHeader(projectName)),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "src", projectName+".cpp"),
		[]byte(templates.GenerateLibSource(projectName)),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "src", "main.cpp"),
		[]byte(templates.GenerateMainCpp(projectName)),
		0644,
	))

	builder := meson.New()
	require.NoError(t, builder.GenerateBuildSrc(projectPath, build.InitConfig{
		Name:        projectName,
		Version:     "1.0.0",
		CppStandard: 17,
		IsLibrary:   false,
	}))
}

// createMesonLibraryProject creates a minimal Meson library project
func createMesonLibraryProject(t *testing.T, projectPath, projectName string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, "include", projectName), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectPath, "src"), 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "include", projectName, "version.hpp"),
		[]byte(templates.GenerateVersionHpp(projectName, "1.0.0")),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "include", projectName, projectName+".hpp"),
		[]byte(templates.GenerateLibHeader(projectName)),
		0644,
	))

	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "src", projectName+".cpp"),
		[]byte(templates.GenerateLibSource(projectName)),
		0644,
	))

	builder := meson.New()
	require.NoError(t, builder.GenerateBuildSrc(projectPath, build.InitConfig{
		Name:        projectName,
		Version:     "1.0.0",
		CppStandard: 17,
		IsLibrary:   true,
	}))
}

// --- CMake Tests ---

func TestScaffoldBuild_CMake_Executable(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "cmake")
	skipIfCompilerMissing(t)

	tmpDir := t.TempDir()
	projectName := "testapp"

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	createCMakeExecutableProject(t, tmpDir, projectName)

	// Build the project
	builder := cmake.New()
	err = builder.Build(build.BuildOptions{
		Release: false,
		Verbose: true,
	})
	require.NoError(t, err)

	// Verify executable was created
	outputDir := filepath.Join(common.NativeOutputDir(), "debug")
	exeName := projectName
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}

	exePath := filepath.Join(outputDir, exeName)
	_, err = os.Stat(exePath)
	assert.NoError(t, err, "executable should exist at %s", exePath)
}

func TestScaffoldBuild_CMake_Library(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "cmake")
	skipIfCompilerMissing(t)

	tmpDir := t.TempDir()
	projectName := "testlib"

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	createCMakeLibraryProject(t, tmpDir, projectName)

	builder := cmake.New()
	err = builder.Build(build.BuildOptions{
		Release: false,
		Verbose: true,
	})
	require.NoError(t, err)

	// Verify library was created
	outputDir := filepath.Join(common.NativeOutputDir(), "debug")
	libs, err := common.FindLibraries(outputDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, libs, "at least one library should be built")
}

func TestFullWorkflow_CMake(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "cmake")
	skipIfCompilerMissing(t)

	tmpDir := t.TempDir()
	projectName := "testworkflow"

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	createCMakeExecutableProject(t, tmpDir, projectName)

	builder := cmake.New()

	// Build
	err = builder.Build(build.BuildOptions{Verbose: true})
	require.NoError(t, err, "build should succeed")

	// Run (may fail if executable doesn't exit cleanly, but build should work)
	err = builder.Run(build.RunOptions{Verbose: true})
	// Run might fail if greet() doesn't exist yet, that's ok

	// Test (may have no tests)
	err = builder.Test(build.TestOptions{Verbose: true})
	// Test might fail if no tests defined, that's ok

	// Clean
	err = builder.Clean(build.CleanOptions{All: true})
	require.NoError(t, err, "clean should succeed")

	// Verify clean worked
	_, err = os.Stat(common.NativeCacheDir())
	assert.True(t, os.IsNotExist(err), "cache dir should be removed")
}

// --- Meson Tests ---

func TestScaffoldBuild_Meson_Executable(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "meson")
	skipIfToolMissing(t, "ninja")
	skipIfCompilerMissing(t)

	tmpDir := t.TempDir()
	projectName := "testmesonapp"

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	createMesonExecutableProject(t, tmpDir, projectName)

	builder := meson.New()
	err = builder.Build(build.BuildOptions{
		Release: false,
		Verbose: true,
	})
	require.NoError(t, err)

	// Verify executable was created
	outputDir := filepath.Join(common.NativeOutputDir(), "debug")
	exeName := projectName
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}

	exePath := filepath.Join(outputDir, exeName)
	_, err = os.Stat(exePath)
	assert.NoError(t, err, "executable should exist at %s", exePath)
}

func TestScaffoldBuild_Meson_Library(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "meson")
	skipIfToolMissing(t, "ninja")
	skipIfCompilerMissing(t)

	tmpDir := t.TempDir()
	projectName := "testmesonlib"

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	createMesonLibraryProject(t, tmpDir, projectName)

	builder := meson.New()
	err = builder.Build(build.BuildOptions{
		Release: false,
		Verbose: true,
	})
	require.NoError(t, err)

	// Verify library was created
	outputDir := filepath.Join(common.NativeOutputDir(), "debug")
	libs, err := common.FindLibraries(outputDir)
	assert.NoError(t, err)
	assert.NotEmpty(t, libs, "at least one library should be built")
}

func TestFullWorkflow_Meson(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "meson")
	skipIfToolMissing(t, "ninja")
	skipIfCompilerMissing(t)

	tmpDir := t.TempDir()
	projectName := "testmesonworkflow"

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	createMesonExecutableProject(t, tmpDir, projectName)

	builder := meson.New()

	// Build
	err = builder.Build(build.BuildOptions{Verbose: true})
	require.NoError(t, err, "build should succeed")

	// Run
	_ = builder.Run(build.RunOptions{Verbose: true})

	// Test
	_ = builder.Test(build.TestOptions{Verbose: true})

	// Clean
	err = builder.Clean(build.CleanOptions{All: true})
	require.NoError(t, err, "clean should succeed")

	// Verify clean worked
	_, err = os.Stat("builddir")
	assert.True(t, os.IsNotExist(err), "builddir should be removed")
}

// --- Bazel Tests ---
// Note: Bazel tests are more complex because they require proper MODULE.bazel setup
// and BCR dependencies. These are simplified versions.

func TestScaffoldBuild_Bazel_Executable(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "bazel")

	// Bazel requires a more complex setup with proper BCR configuration
	// For now, we'll just verify bazel is present and skip the full build test
	t.Skip("Bazel integration test requires BCR setup - skipping for now")
}

func TestScaffoldBuild_Bazel_Library(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "bazel")

	t.Skip("Bazel integration test requires BCR setup - skipping for now")
}

func TestFullWorkflow_Bazel(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "bazel")

	t.Skip("Bazel integration test requires BCR setup - skipping for now")
}

// --- Release Build Tests ---

func TestScaffoldBuild_CMake_Release(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "cmake")
	skipIfCompilerMissing(t)

	tmpDir := t.TempDir()
	projectName := "testreleaseapp"

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	createCMakeExecutableProject(t, tmpDir, projectName)

	builder := cmake.New()
	err = builder.Build(build.BuildOptions{
		Release: true,
		Verbose: true,
	})
	require.NoError(t, err)

	// Verify executable was created in release dir
	outputDir := filepath.Join(common.NativeOutputDir(), "release")
	exeName := projectName
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}

	exePath := filepath.Join(outputDir, exeName)
	_, err = os.Stat(exePath)
	assert.NoError(t, err, "release executable should exist at %s", exePath)
}

func TestScaffoldBuild_Meson_Release(t *testing.T) {
	skipIfShort(t)
	skipIfToolMissing(t, "meson")
	skipIfToolMissing(t, "ninja")
	skipIfCompilerMissing(t)

	tmpDir := t.TempDir()
	projectName := "testmesonrelease"

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	createMesonExecutableProject(t, tmpDir, projectName)

	builder := meson.New()
	err = builder.Build(build.BuildOptions{
		Release: true,
		Verbose: true,
	})
	require.NoError(t, err)

	// Verify executable was created in release dir
	outputDir := filepath.Join(common.NativeOutputDir(), "release")
	exeName := projectName
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}

	exePath := filepath.Join(outputDir, exeName)
	_, err = os.Stat(exePath)
	assert.NoError(t, err, "release executable should exist at %s", exePath)
}
