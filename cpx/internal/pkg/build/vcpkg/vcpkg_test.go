package vcpkg

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ozacod/cpx/internal/pkg/build/common"
	build "github.com/ozacod/cpx/internal/pkg/build/interfaces"
	"github.com/ozacod/cpx/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess isn't a real test. It's used as a helper process
// for mocking exec.Command.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func TestClean(t *testing.T) {
	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create CMake project files
	require.NoError(t, os.WriteFile("CMakeLists.txt", []byte("cmake_minimum_required(VERSION 3.16)"), 0644))

	tests := []struct {
		name          string
		all           bool
		expectRemoved []string
	}{
		{
			name:          "Clean without all flag",
			all:           false,
			expectRemoved: []string{".bin/native"},
		},
		{
			name:          "Clean with all flag",
			all:           true,
			expectRemoved: []string{".bin/native", ".bin/ci", "out", "cmake-build-debug", "build-release"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate dirs for each test
			_ = os.MkdirAll(".bin/native", 0755)
			_ = os.MkdirAll(".bin/ci", 0755)
			_ = os.MkdirAll("out", 0755)
			_ = os.MkdirAll("cmake-build-debug", 0755)
			_ = os.MkdirAll("build-release", 0755)

			builder := New()
			err := builder.Clean(build.CleanOptions{All: tt.all})
			assert.NoError(t, err)

			// Verify expected directories were removed
			for _, dir := range tt.expectRemoved {
				_, err = os.Stat(dir)
				assert.True(t, os.IsNotExist(err), "%s should be removed", dir)
			}
		})
	}
}

func mockExecCommand(capturedArgs *[][]string) func(string, ...string) *exec.Cmd {
	return func(name string, arg ...string) *exec.Cmd {
		args := append([]string{name}, arg...)
		*capturedArgs = append(*capturedArgs, args)

		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
}

func setupTestConfig(tmpDir string) *Builder {
	b := New()
	cfg := &config.GlobalConfig{
		VcpkgRoot: tmpDir,
	}
	b.globalConfig = cfg
	return b
}

func TestBuild(t *testing.T) {
	oldExecCommand := execCommand
	oldCommonExecCommand := common.ExecCommand
	defer func() {
		execCommand = oldExecCommand
		common.ExecCommand = oldCommonExecCommand
	}()

	var capturedArgs [][]string
	mockCmd := mockExecCommand(&capturedArgs)
	execCommand = mockCmd
	common.ExecCommand = mockCmd

	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// Mock vcpkg executable
	vcpkgPath := filepath.Join(tmpDir, "vcpkg")
	_ = os.WriteFile(vcpkgPath, []byte(""), 0755)
	_ = os.WriteFile("CMakeLists.txt", []byte("project(test)"), 0644)

	// Build dir and dummy cache to avoid configure failing
	outDir := build.GetOutputDir(true, "", "")
	cacheDir := filepath.Join(".cache", "native", outDir)
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "CMakeCache.txt"), []byte(""), 0644)

	builder := setupTestConfig(tmpDir)

	err := builder.Build(build.BuildOptions{
		Release: true,
		Clean:   true,
	})
	assert.NoError(t, err)

	// Verify cmake was called for build (configure might be skipped if cache exists)
	foundBuild := false
	for _, args := range capturedArgs {
		if args[0] == "cmake" && len(args) > 1 && args[1] == "--build" {
			foundBuild = true
			break
		}
	}
	assert.True(t, foundBuild, "cmake build should be called")
}

func TestTest(t *testing.T) {
	oldExecCommand := execCommand
	oldCommonExecCommand := common.ExecCommand
	defer func() {
		execCommand = oldExecCommand
		common.ExecCommand = oldCommonExecCommand
	}()

	var capturedArgs [][]string
	mockCmd := mockExecCommand(&capturedArgs)
	execCommand = mockCmd
	common.ExecCommand = mockCmd

	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// Mock vcpkg executable
	vcpkgPath := filepath.Join(tmpDir, "vcpkg")
	_ = os.WriteFile(vcpkgPath, []byte(""), 0755)
	_ = os.WriteFile("CMakeLists.txt", []byte("project(test)"), 0644)

	// Ensure cache build dir exists for TestTest and has cache
	testCacheDir := ".cache/native/test"
	_ = os.MkdirAll(testCacheDir, 0755)
	_ = os.WriteFile(filepath.Join(testCacheDir, "CMakeCache.txt"), []byte(""), 0644)

	builder := setupTestConfig(tmpDir)

	err := builder.Test(build.TestOptions{
		Verbose: true,
	})
	assert.NoError(t, err)

	foundCtest := false
	for _, args := range capturedArgs {
		if args[0] == "ctest" {
			foundCtest = true
			break
		}
	}
	assert.True(t, foundCtest, "ctest should be called")
}

func TestRun(t *testing.T) {
	oldExecCommand := execCommand
	oldCommonExecCommand := common.ExecCommand
	defer func() {
		execCommand = oldExecCommand
		common.ExecCommand = oldCommonExecCommand
	}()

	var capturedArgs [][]string
	mockCmd := mockExecCommand(&capturedArgs)
	execCommand = mockCmd
	common.ExecCommand = mockCmd

	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// Mock vcpkg executable
	vcpkgPath := filepath.Join(tmpDir, "vcpkg")
	_ = os.WriteFile(vcpkgPath, []byte(""), 0755)
	_ = os.WriteFile("CMakeLists.txt", []byte("project(test)"), 0644)

	// Mock build output and cache
	outDir := build.GetOutputDir(false, "", "")
	cacheDir := filepath.Join(".cache", "native", outDir)
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "CMakeCache.txt"), []byte(""), 0644)

	binDir := filepath.Join(".bin", "native", outDir)
	_ = os.MkdirAll(binDir, 0755)
	exePath := filepath.Join(binDir, "test")
	_ = os.WriteFile(exePath, []byte(""), 0755)

	builder := setupTestConfig(tmpDir)

	err := builder.Run(build.RunOptions{})
	assert.NoError(t, err)

	foundRun := false
	for _, args := range capturedArgs {
		if filepath.Base(args[0]) == "test" {
			foundRun = true
			break
		}
	}
	assert.True(t, foundRun, "executable should be run")
}

func TestAddDependency(t *testing.T) {
	oldExecCommand := execCommand
	oldCommonExecCommand := common.ExecCommand
	defer func() {
		execCommand = oldExecCommand
		common.ExecCommand = oldCommonExecCommand
	}()

	var capturedArgs [][]string
	mockCmd := mockExecCommand(&capturedArgs)
	execCommand = mockCmd
	common.ExecCommand = mockCmd

	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	vcpkgPath := filepath.Join(tmpDir, "vcpkg")
	_ = os.WriteFile(vcpkgPath, []byte(""), 0755)

	builder := setupTestConfig(tmpDir)

	err := builder.AddDependency("zlib", "1.2.11")
	assert.NoError(t, err)

	foundVcpkgAdd := false
	for _, args := range capturedArgs {
		if filepath.Base(args[0]) == "vcpkg" && len(args) > 1 && args[1] == "add" && args[2] == "port" && args[3] == "zlib" {
			foundVcpkgAdd = true
			break
		}
	}
	assert.True(t, foundVcpkgAdd, "vcpkg add port zlib should be called")
}
