package cmake

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ozacod/cpx/internal/build/common"
	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess isn't a real test. It's used as a helper process
// for mocking exec.Command.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if out := os.Getenv("MOCK_OUTPUT"); out != "" {
		fmt.Print(out)
	}
	os.Exit(0)
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

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create CMakeLists.txt
	require.NoError(t, os.WriteFile("CMakeLists.txt", []byte(`
cmake_minimum_required(VERSION 3.14)
project(testproj VERSION 1.0)
add_executable(testproj src/main.cpp)
`), 0644))

	tests := []struct {
		name          string
		release       bool
		optLevel      string
		target        string
		clean         bool
		outputMode    build.OutputMode
		sanitizer     string
		wantBuildType string
	}{
		{
			name:          "Debug build",
			release:       false,
			wantBuildType: "Debug",
		},
		{
			name:          "Release build",
			release:       true,
			wantBuildType: "Release",
		},
		{
			name:          "O3 optimization",
			optLevel:      "3",
			wantBuildType: "Release",
		},
		{
			name:          "Build with target",
			release:       false,
			target:        "mylib",
			wantBuildType: "Debug",
		},
		{
			name:          "Clean build",
			release:       false,
			clean:         true,
			wantBuildType: "Debug",
		},
		{
			name:          "ASan build",
			release:       false,
			sanitizer:     "asan",
			wantBuildType: "Debug",
		},
	}

	builder := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedArgs = nil

			// Clean up any previous build artifacts
			require.NoError(t, os.RemoveAll(".cache"))
			require.NoError(t, os.RemoveAll(".bin"))

			opts := build.BuildOptions{
				Release:    tt.release,
				OptLevel:   tt.optLevel,
				Target:     tt.target,
				Clean:      tt.clean,
				OutputMode: tt.outputMode,
				Sanitizer:  tt.sanitizer,
			}

			err := builder.Build(opts)
			assert.NoError(t, err)

			// Check that cmake configure was called
			foundConfigure := false
			foundBuild := false
			for _, args := range capturedArgs {
				if len(args) >= 2 && args[0] == "cmake" && args[1] == "-B" {
					foundConfigure = true
					assert.Contains(t, args, "-DCMAKE_BUILD_TYPE="+tt.wantBuildType)
					if tt.sanitizer == "asan" {
						// Check for sanitizer flags
						for _, arg := range args {
							if arg == "-DCMAKE_CXX_FLAGS=-fsanitize=address -fno-omit-frame-pointer" {
								break
							}
						}
					}
				}
				if len(args) >= 3 && args[0] == "cmake" && args[1] == "--build" {
					foundBuild = true
					if tt.target != "" {
						assert.Contains(t, args, "--target")
						assert.Contains(t, args, tt.target)
					}
				}
			}
			assert.True(t, foundConfigure, "cmake configure should be called")
			assert.True(t, foundBuild, "cmake build should be called")
		})
	}
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

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create CMakeLists.txt
	require.NoError(t, os.WriteFile("CMakeLists.txt", []byte(`
cmake_minimum_required(VERSION 3.14)
project(myapp VERSION 1.0)
add_executable(myapp src/main.cpp)
`), 0644))

	// Create output directory with mock executable
	outputDir := filepath.Join(".bin", "native", "debug")
	require.NoError(t, os.MkdirAll(outputDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "myapp"), []byte("#!/bin/sh\necho hello"), 0755))

	builder := New()

	err = builder.Run(build.RunOptions{
		Release: false,
		Target:  "myapp",
		Verbose: true,
	})
	assert.NoError(t, err)

	// Verify the executable was run
	require.GreaterOrEqual(t, len(capturedArgs), 1)
	lastCmd := capturedArgs[len(capturedArgs)-1]
	assert.Contains(t, lastCmd[0], "myapp")
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

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create CMakeLists.txt
	require.NoError(t, os.WriteFile("CMakeLists.txt", []byte(`
cmake_minimum_required(VERSION 3.14)
project(myapp VERSION 1.0)
`), 0644))

	builder := New()

	err = builder.Test(build.TestOptions{
		Verbose: true,
		Filter:  "mytest",
	})
	assert.NoError(t, err)

	// Check that cmake configure was called with testing enabled
	foundConfigure := false
	foundCtest := false
	for _, args := range capturedArgs {
		if len(args) >= 2 && args[0] == "cmake" && args[1] == "-B" {
			foundConfigure = true
			assert.Contains(t, args, "-DENABLE_TESTING=ON")
		}
		if args[0] == "ctest" {
			foundCtest = true
			assert.Contains(t, args, "-R")
			assert.Contains(t, args, "mytest")
		}
	}
	assert.True(t, foundConfigure, "cmake configure should be called")
	assert.True(t, foundCtest, "ctest should be called")
}

func TestBench(t *testing.T) {
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

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create CMakeLists.txt
	require.NoError(t, os.WriteFile("CMakeLists.txt", []byte(`
cmake_minimum_required(VERSION 3.14)
project(myapp VERSION 1.0)
`), 0644))

	// Create output directory with mock benchmark executable
	// The Bench function looks in .cache/native/bench/bench/ and .cache/native/bench/
	outputDir := filepath.Join(common.NativeCacheDir(), "bench")
	require.NoError(t, os.MkdirAll(outputDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "myapp_bench"), []byte("#!/bin/sh\necho bench"), 0755))

	builder := New()

	err = builder.Bench(build.BenchOptions{
		Verbose: true,
		Target:  "myapp_bench",
	})
	assert.NoError(t, err)

	// Verify the benchmark was run
	require.GreaterOrEqual(t, len(capturedArgs), 1)
	lastCmd := capturedArgs[len(capturedArgs)-1]
	assert.Contains(t, lastCmd[0], "myapp_bench")
}

func TestClean(t *testing.T) {
	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create CMakeLists.txt
	require.NoError(t, os.WriteFile("CMakeLists.txt", []byte("project(test)"), 0644))

	tests := []struct {
		name          string
		all           bool
		expectRemoved []string
	}{
		{
			name:          "Clean without all flag",
			all:           false,
			expectRemoved: []string{common.NativeCacheDir(), common.NativeOutputDir()},
		},
		{
			name:          "Clean with all flag",
			all:           true,
			expectRemoved: []string{common.NativeCacheDir(), common.NativeOutputDir(), "build", "build-release"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate dirs for each test
			_ = os.MkdirAll(filepath.Join(common.NativeCacheDir(), "debug"), 0755)
			_ = os.MkdirAll(filepath.Join(common.NativeOutputDir(), "debug"), 0755)
			_ = os.MkdirAll("build", 0755)
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

func TestAddDependency(t *testing.T) {
	builder := New()

	// AddDependency should not error, just print guidance
	err := builder.AddDependency("zlib", "1.2.13")
	assert.NoError(t, err)
}

func TestRemoveDependency(t *testing.T) {
	builder := New()

	// RemoveDependency should return an error for cmake-only projects
	err := builder.RemoveDependency("zlib")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "don't have built-in dependency management")
}

func TestListDependencies(t *testing.T) {
	builder := New()

	// ListDependencies returns nil for cmake-only projects
	deps, err := builder.ListDependencies()
	assert.NoError(t, err)
	assert.Nil(t, deps)
}

func TestSearchDependencies(t *testing.T) {
	builder := New()

	// SearchDependencies should return an error for cmake-only projects
	deps, err := builder.SearchDependencies("zlib")
	assert.Error(t, err)
	assert.Nil(t, deps)
}

func TestDependencyInfo(t *testing.T) {
	builder := New()

	// DependencyInfo should return an error for cmake-only projects
	info, err := builder.DependencyInfo("zlib")
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestName(t *testing.T) {
	builder := New()
	assert.Equal(t, "cmake", builder.Name())
}

func TestListTargets(t *testing.T) {
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	// Mock cmake to fail so it falls back to parsing CMakeLists.txt
	// The cmake --build --target help output format varies by generator
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cmd := exec.Command("false") // This will fail
		return cmd
	}

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// Create build directory
	_ = os.MkdirAll(filepath.Join(common.NativeCacheDir(), "debug"), 0755)

	// Create CMakeLists.txt with targets
	require.NoError(t, os.WriteFile("CMakeLists.txt", []byte(`
cmake_minimum_required(VERSION 3.14)
project(test)
add_executable(myapp src/main.cpp)
add_library(mylib src/lib.cpp)
`), 0644))

	builder := New()
	targets, err := builder.ListTargets()
	assert.NoError(t, err)
	assert.Contains(t, targets, "myapp")
	assert.Contains(t, targets, "mylib")
}

func TestListTargetsFallback(t *testing.T) {
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	// Mock cmake to fail so it falls back to parsing CMakeLists.txt
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cmd := exec.Command("false") // This will fail
		return cmd
	}

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// Create build directory
	_ = os.MkdirAll(filepath.Join(common.NativeCacheDir(), "debug"), 0755)

	// Create CMakeLists.txt with targets
	require.NoError(t, os.WriteFile("CMakeLists.txt", []byte(`
cmake_minimum_required(VERSION 3.14)
project(test)
add_executable(myapp src/main.cpp)
add_library(mylib src/lib.cpp)
`), 0644))

	builder := New()
	targets, err := builder.ListTargets()
	assert.NoError(t, err)
	assert.Contains(t, targets, "myapp")
	assert.Contains(t, targets, "mylib")
}

func TestGenerateBuildSrc(t *testing.T) {
	tmpDir := t.TempDir()

	builder := New()
	err := builder.GenerateBuildSrc(tmpDir, build.InitConfig{
		Name:          "testproj",
		Version:       "1.0.0",
		CppStandard:   17,
		IsLibrary:     false,
		TestFramework: "googletest",
		Benchmark:     "google-benchmark",
	})
	assert.NoError(t, err)

	// Verify CMakeLists.txt was created
	_, err = os.Stat(filepath.Join(tmpDir, "CMakeLists.txt"))
	assert.NoError(t, err)
}

func TestGenerateBuildTest(t *testing.T) {
	tmpDir := t.TempDir()

	builder := New()

	// Test with no test framework
	err := builder.GenerateBuildTest(tmpDir, build.InitConfig{
		Name:          "testproj",
		TestFramework: "none",
	})
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "tests", "CMakeLists.txt"))
	assert.True(t, os.IsNotExist(err), "tests/CMakeLists.txt should not be created when no framework")

	// Test with googletest
	err = builder.GenerateBuildTest(tmpDir, build.InitConfig{
		Name:          "testproj",
		TestFramework: "googletest",
	})
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "tests", "CMakeLists.txt"))
	assert.NoError(t, err)
}

func TestGenerateBuildBench(t *testing.T) {
	tmpDir := t.TempDir()

	builder := New()

	// Test with no benchmark framework
	err := builder.GenerateBuildBench(tmpDir, build.InitConfig{
		Name:      "testproj",
		Benchmark: "none",
	})
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "bench", "CMakeLists.txt"))
	assert.True(t, os.IsNotExist(err), "bench/CMakeLists.txt should not be created when no framework")

	// Test with google-benchmark
	err = builder.GenerateBuildBench(tmpDir, build.InitConfig{
		Name:      "testproj",
		Benchmark: "google-benchmark",
	})
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "bench", "CMakeLists.txt"))
	assert.NoError(t, err)
}

func TestGenerateGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	builder := New()
	err := builder.GenerateGitignore(tmpDir)
	assert.NoError(t, err)

	// Verify .gitignore was created
	content, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	assert.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestBuilderImplementsInterface(t *testing.T) {
	// Compile-time check that CMakeBuilder implements BuildSystem
	var _ build.BuildSystem = (*CMakeBuilder)(nil)
}
