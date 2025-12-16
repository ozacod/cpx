package quality

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGitTrackedCppFiles_InGitRepo(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	cmd := exec.Command("git", "init")
	require.NoError(t, cmd.Run())

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Test User")
	require.NoError(t, cmd.Run())

	// Create some C++ files
	require.NoError(t, os.MkdirAll("src", 0755))
	require.NoError(t, os.WriteFile("src/main.cpp", []byte("int main() {}"), 0644))
	require.NoError(t, os.WriteFile("src/utils.hpp", []byte("#pragma once"), 0644))
	require.NoError(t, os.WriteFile("src/helper.cc", []byte("// helper"), 0644))
	require.NoError(t, os.WriteFile("readme.txt", []byte("readme"), 0644)) // Non-C++ file

	// Add files to git
	cmd = exec.Command("git", "add", ".")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "initial")
	require.NoError(t, cmd.Run())

	// Test GetGitTrackedCppFiles
	files, err := GetGitTrackedCppFiles()
	require.NoError(t, err)

	// Should contain C++ files but not the txt file
	assert.Contains(t, files, "src/main.cpp")
	assert.Contains(t, files, "src/utils.hpp")
	assert.Contains(t, files, "src/helper.cc")

	// Should not contain non-C++ files
	for _, file := range files {
		assert.NotEqual(t, "readme.txt", file)
	}
}

func TestGetGitTrackedCppFiles_NotInGitRepo(t *testing.T) {
	// Create a temporary directory without a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Should return error when not in git repo
	_, err = GetGitTrackedCppFiles()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in a git repository")
}

func TestGetGitTrackedCppFiles_AllCppExtensions(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	cmd := exec.Command("git", "init")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Test User")
	require.NoError(t, cmd.Run())

	// Create files with all supported C++ extensions
	cppFiles := []string{
		"file.cpp", "file.cxx", "file.cc", "file.c++",
		"header.hpp", "header.hxx", "header.hh", "header.h++",
		"source.c", "header.h",
		"module.cppm", "module.ixx",
	}

	for _, f := range cppFiles {
		require.NoError(t, os.WriteFile(f, []byte("// content"), 0644))
	}

	// Add files to git
	cmd = exec.Command("git", "add", ".")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "initial")
	require.NoError(t, cmd.Run())

	// Test GetGitTrackedCppFiles
	files, err := GetGitTrackedCppFiles()
	require.NoError(t, err)

	// All C++ files should be tracked
	assert.Equal(t, len(cppFiles), len(files))
	for _, expected := range cppFiles {
		assert.Contains(t, files, expected)
	}
}

func TestFilterGitTrackedFiles_WithTargets(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	cmd := exec.Command("git", "init")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Test User")
	require.NoError(t, cmd.Run())

	// Create files in different directories
	require.NoError(t, os.MkdirAll("src", 0755))
	require.NoError(t, os.MkdirAll("test", 0755))
	require.NoError(t, os.MkdirAll("examples", 0755))

	require.NoError(t, os.WriteFile("src/main.cpp", []byte("int main() {}"), 0644))
	require.NoError(t, os.WriteFile("src/utils.cpp", []byte("// utils"), 0644))
	require.NoError(t, os.WriteFile("test/test_main.cpp", []byte("// test"), 0644))
	require.NoError(t, os.WriteFile("examples/example.cpp", []byte("// example"), 0644))

	// Add files to git
	cmd = exec.Command("git", "add", ".")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "initial")
	require.NoError(t, cmd.Run())

	// Test filtering to specific target
	srcAbs, _ := filepath.Abs("src")
	files, err := FilterGitTrackedFiles([]string{srcAbs})
	require.NoError(t, err)

	// Should only contain files from src directory
	assert.Equal(t, 2, len(files))
	for _, file := range files {
		absFile, _ := filepath.Abs(file)
		assert.True(t, filepath.HasPrefix(absFile, srcAbs) || absFile == srcAbs,
			"File %s should be in src directory", file)
	}
}

func TestFilterGitTrackedFiles_DotTarget(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	cmd := exec.Command("git", "init")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Test User")
	require.NoError(t, cmd.Run())

	// Create some files
	require.NoError(t, os.WriteFile("main.cpp", []byte("int main() {}"), 0644))
	require.NoError(t, os.MkdirAll("src", 0755))
	require.NoError(t, os.WriteFile("src/lib.cpp", []byte("// lib"), 0644))

	// Add files to git
	cmd = exec.Command("git", "add", ".")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "initial")
	require.NoError(t, cmd.Run())

	// Using "." as target should return all files
	files, err := FilterGitTrackedFiles([]string{"."})
	require.NoError(t, err)

	// Should contain all C++ files
	assert.Equal(t, 2, len(files))
}

func TestFilterGitTrackedFiles_NotInGitRepo(t *testing.T) {
	// Create a temporary directory without a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Should return error when not in git repo
	_, err = FilterGitTrackedFiles([]string{"."})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in a git repository")
}

func TestGetGitTrackedCppFiles_SkipsDeletedFiles(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	cmd := exec.Command("git", "init")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Test User")
	require.NoError(t, cmd.Run())

	// Create and commit a file
	require.NoError(t, os.WriteFile("main.cpp", []byte("int main() {}"), 0644))
	require.NoError(t, os.WriteFile("deleted.cpp", []byte("// to delete"), 0644))

	cmd = exec.Command("git", "add", ".")
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "commit", "-m", "initial")
	require.NoError(t, cmd.Run())

	// Delete the file (but it's still in git ls-files)
	require.NoError(t, os.Remove("deleted.cpp"))

	// Test GetGitTrackedCppFiles
	files, err := GetGitTrackedCppFiles()
	require.NoError(t, err)

	// Should only contain existing files
	assert.Contains(t, files, "main.cpp")
	assert.NotContains(t, files, "deleted.cpp")
}

func TestGetGitTrackedCppFiles_EmptyRepo(t *testing.T) {
	// Create a temporary directory with an empty git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo without any commits
	cmd := exec.Command("git", "init")
	require.NoError(t, cmd.Run())

	// Test GetGitTrackedCppFiles on empty repo
	files, err := GetGitTrackedCppFiles()
	require.NoError(t, err)

	// Should return empty slice
	assert.Empty(t, files)
}
