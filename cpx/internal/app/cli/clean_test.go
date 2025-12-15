package cli

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanBazel(t *testing.T) {
	// Mock execCommand
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func(dir string) {
		err := os.Chdir(dir)
		if err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}(oldWd)
	require.NoError(t, os.Chdir(tmpDir))

	// Create Bazel project files
	require.NoError(t, os.WriteFile("MODULE.bazel", []byte("module(name = \"test\")"), 0644))
	require.NoError(t, os.MkdirAll("build", 0755))
	require.NoError(t, os.MkdirAll(".bin", 0755))
	require.NoError(t, os.MkdirAll(".out", 0755))
	require.NoError(t, os.MkdirAll("bazel-bin", 0755))
	require.NoError(t, os.MkdirAll("bazel-out", 0755))

	tests := []struct {
		name string
		all  bool
	}{
		{
			name: "Clean without all flag",
			all:  false,
		},
		{
			name: "Clean with all flag",
			all:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate dirs for each test
			err := os.MkdirAll("build", 0755)
			if err != nil {
				t.Fatalf("%v", err)
			}
			err = os.MkdirAll(".bin", 0755)
			if err != nil {
				t.Fatalf("%v", err)
			}
			err = os.MkdirAll("bazel-bin", 0755)
			if err != nil {
				t.Fatalf("%v", err)
			}

			err = cleanBazel(tt.all)
			assert.NoError(t, err)

			// Verify build directory was removed
			_, err = os.Stat("build")
			assert.True(t, os.IsNotExist(err), "build directory should be removed")

			// Verify .bin was removed
			_, err = os.Stat(".bin")
			assert.True(t, os.IsNotExist(err), ".bin should be removed")

			// Verify bazel-bin was removed
			_, err = os.Stat("bazel-bin")
			assert.True(t, os.IsNotExist(err), "bazel-bin should be removed")
		})
	}
}

func TestCleanMeson(t *testing.T) {
	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create Meson project files
	require.NoError(t, os.WriteFile("meson.build", []byte("project('test', 'cpp')"), 0644))
	require.NoError(t, os.MkdirAll("builddir", 0755))
	require.NoError(t, os.MkdirAll("build", 0755))
	require.NoError(t, os.MkdirAll("subprojects/packagecache", 0755))
	require.NoError(t, os.MkdirAll("build-release", 0755))

	tests := []struct {
		name          string
		all           bool
		expectRemoved []string
	}{
		{
			name:          "Clean without all flag",
			all:           false,
			expectRemoved: []string{"builddir", "build"},
		},
		{
			name:          "Clean with all flag",
			all:           true,
			expectRemoved: []string{"builddir", "build", "subprojects/packagecache", "build-release"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate dirs for each test
			_ = os.MkdirAll("builddir", 0755)
			_ = os.MkdirAll("build", 0755)
			_ = os.MkdirAll("subprojects/packagecache", 0755)
			_ = os.MkdirAll("build-release", 0755)

			err := cleanMeson(tt.all)
			assert.NoError(t, err)

			// Verify expected directories were removed
			for _, dir := range tt.expectRemoved {
				_, err = os.Stat(dir)
				assert.True(t, os.IsNotExist(err), "%s should be removed", dir)
			}
		})
	}
}

func TestCleanCMake(t *testing.T) {
	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create CMake project files
	require.NoError(t, os.WriteFile("CMakeLists.txt", []byte("cmake_minimum_required(VERSION 3.16)"), 0644))
	require.NoError(t, os.MkdirAll("build", 0755))
	require.NoError(t, os.MkdirAll("out", 0755))
	require.NoError(t, os.MkdirAll("cmake-build-debug", 0755))
	require.NoError(t, os.MkdirAll("build-release", 0755))

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

			err := cleanCMake(tt.all)
			assert.NoError(t, err)

			// Verify expected directories were removed
			for _, dir := range tt.expectRemoved {
				_, err = os.Stat(dir)
				assert.True(t, os.IsNotExist(err), "%s should be removed", dir)
			}
		})
	}
}

func TestRemoveDir(t *testing.T) {
	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	tests := []struct {
		name      string
		createDir bool
		path      string
	}{
		{
			name:      "Remove existing directory",
			createDir: true,
			path:      "testdir",
		},
		{
			name:      "Remove non-existent directory (should not error)",
			createDir: false,
			path:      "nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createDir {
				require.NoError(t, os.MkdirAll(tt.path, 0755))
			}

			// removeDir should not panic or error even if dir doesn't exist
			removeDir(tt.path)

			// Verify directory was removed or didn't exist
			_, err := os.Stat(tt.path)
			assert.True(t, os.IsNotExist(err), "directory should not exist after removeDir")
		})
	}
}

func TestRunClean(t *testing.T) {
	// Mock execCommand for bazel clean
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	tests := []struct {
		name        string
		projectFile string
		createDirs  []string
		all         bool
	}{
		{
			name:        "Clean Bazel project",
			projectFile: "MODULE.bazel",
			createDirs:  []string{"build", ".bin"},
			all:         false,
		},
		{
			name:        "Clean Meson project",
			projectFile: "meson.build",
			createDirs:  []string{"builddir", "build"},
			all:         false,
		},
		{
			name:        "Clean CMake project",
			projectFile: "CMakeLists.txt",
			createDirs:  []string{".bin/native"},
			all:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp dir
			tmpDir := t.TempDir()
			oldWd, err := os.Getwd()
			require.NoError(t, err)
			defer func() { _ = os.Chdir(oldWd) }()
			require.NoError(t, os.Chdir(tmpDir))

			// Create project file
			require.NoError(t, os.WriteFile(tt.projectFile, []byte("test"), 0644))

			// Create directories
			for _, dir := range tt.createDirs {
				require.NoError(t, os.MkdirAll(dir, 0755))
			}

			// Create a mock command
			cmd := CleanCmd()
			if tt.all {
				_ = cmd.Flags().Set("all", "true")
			}

			err = cmd.Execute()
			assert.NoError(t, err)

			// Verify directories were cleaned
			for _, dir := range tt.createDirs {
				// For bazel, some dirs might not be cleaned without the actual bazel command
				// but build dir should be cleaned
				if dir == "build" || dir == "builddir" || dir == ".bin/native" {
					_, err = os.Stat(dir)
					assert.True(t, os.IsNotExist(err), "%s should be removed", dir)
				}
			}
		})
	}
}
