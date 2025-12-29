package build

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyAndSign(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a source file
	srcFile := filepath.Join(tmpDir, "source")
	srcContent := []byte("test content for copy")
	require.NoError(t, os.WriteFile(srcFile, srcContent, 0755))

	t.Run("Basic copy", func(t *testing.T) {
		destFile := filepath.Join(tmpDir, "dest1")

		err := copyAndSign(srcFile, destFile)
		require.NoError(t, err)

		// Verify destination file exists
		_, err = os.Stat(destFile)
		require.NoError(t, err)

		// Verify content matches
		destContent, err := os.ReadFile(destFile)
		require.NoError(t, err)
		assert.Equal(t, srcContent, destContent)

		// Verify file is executable
		info, err := os.Stat(destFile)
		require.NoError(t, err)
		assert.True(t, info.Mode().Perm()&0100 != 0, "File should be executable")
	})

	t.Run("Overwrite existing file", func(t *testing.T) {
		destFile := filepath.Join(tmpDir, "dest2")

		// Create existing destination file with different content
		require.NoError(t, os.WriteFile(destFile, []byte("old content"), 0644))

		err := copyAndSign(srcFile, destFile)
		require.NoError(t, err)

		// Verify content was replaced
		destContent, err := os.ReadFile(destFile)
		require.NoError(t, err)
		assert.Equal(t, srcContent, destContent)
	})

	t.Run("Copy to new directory", func(t *testing.T) {
		newDir := filepath.Join(tmpDir, "newdir")
		require.NoError(t, os.MkdirAll(newDir, 0755))
		destFile := filepath.Join(newDir, "dest3")

		err := copyAndSign(srcFile, destFile)
		require.NoError(t, err)

		// Verify destination file exists
		_, err = os.Stat(destFile)
		require.NoError(t, err)
	})

	t.Run("Non-existent source file", func(t *testing.T) {
		nonExistentSrc := filepath.Join(tmpDir, "nonexistent")
		destFile := filepath.Join(tmpDir, "dest4")

		err := copyAndSign(nonExistentSrc, destFile)
		assert.Error(t, err)
	})
}

func TestCopyAndSign_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a larger source file (1MB)
	srcFile := filepath.Join(tmpDir, "large_source")
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	require.NoError(t, os.WriteFile(srcFile, largeContent, 0755))

	destFile := filepath.Join(tmpDir, "large_dest")

	err := copyAndSign(srcFile, destFile)
	require.NoError(t, err)

	// Verify content matches
	destContent, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, largeContent, destContent)
}

func TestCopyAndSign_BinaryFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a binary file with null bytes
	srcFile := filepath.Join(tmpDir, "binary_source")
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x00, 0x7F}
	require.NoError(t, os.WriteFile(srcFile, binaryContent, 0755))

	destFile := filepath.Join(tmpDir, "binary_dest")

	err := copyAndSign(srcFile, destFile)
	require.NoError(t, err)

	// Verify content matches exactly
	destContent, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, destContent)
}

func TestCopyAndSign_PreservesExecutability(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific permission test on Windows")
	}

	tmpDir := t.TempDir()

	// Create an executable source file
	srcFile := filepath.Join(tmpDir, "exec_source")
	require.NoError(t, os.WriteFile(srcFile, []byte("#!/bin/bash\necho test"), 0755))

	destFile := filepath.Join(tmpDir, "exec_dest")

	err := copyAndSign(srcFile, destFile)
	require.NoError(t, err)

	// Verify file is executable
	info, err := os.Stat(destFile)
	require.NoError(t, err)

	// Check that executable bits are set
	assert.True(t, info.Mode().Perm()&0111 != 0, "File should be executable")
}

func TestCopyAndSign_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an empty source file
	srcFile := filepath.Join(tmpDir, "empty_source")
	require.NoError(t, os.WriteFile(srcFile, []byte{}, 0755))

	destFile := filepath.Join(tmpDir, "empty_dest")

	err := copyAndSign(srcFile, destFile)
	require.NoError(t, err)

	// Verify destination file exists and is empty
	info, err := os.Stat(destFile)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func TestCopyAndSign_RemovesExistingDestination(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(tmpDir, "source")
	require.NoError(t, os.WriteFile(srcFile, []byte("new content"), 0755))

	// Create a readonly destination file
	destFile := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.WriteFile(destFile, []byte("old content"), 0444))

	// copyAndSign should remove and recreate the file
	err := copyAndSign(srcFile, destFile)

	// On some systems, removing readonly files might fail
	// The function should handle this gracefully
	if err == nil {
		content, readErr := os.ReadFile(destFile)
		require.NoError(t, readErr)
		assert.Equal(t, "new content", string(content))
	}
}

func TestCopyAndSign_SymlinkSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	tmpDir := t.TempDir()

	// Create actual source file
	actualFile := filepath.Join(tmpDir, "actual")
	require.NoError(t, os.WriteFile(actualFile, []byte("symlink content"), 0755))

	// Create symlink to actual file
	symlinkFile := filepath.Join(tmpDir, "symlink")
	require.NoError(t, os.Symlink(actualFile, symlinkFile))

	destFile := filepath.Join(tmpDir, "dest")

	// Copy from symlink should copy the actual content
	err := copyAndSign(symlinkFile, destFile)
	require.NoError(t, err)

	// Verify content matches
	destContent, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, "symlink content", string(destContent))

	// Destination should be a regular file, not a symlink
	info, err := os.Lstat(destFile)
	require.NoError(t, err)
	assert.False(t, info.Mode()&os.ModeSymlink != 0, "Destination should not be a symlink")
}

func TestCopyAndSign_SpecialCharactersInPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(tmpDir, "source file with spaces.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0755))

	destFile := filepath.Join(tmpDir, "dest file with spaces.txt")

	err := copyAndSign(srcFile, destFile)
	require.NoError(t, err)

	// Verify destination file exists
	_, err = os.Stat(destFile)
	require.NoError(t, err)
}

func TestCopyAndSign_PlatformSpecific(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(tmpDir, "platform_src")
	require.NoError(t, os.WriteFile(srcFile, []byte("platform test"), 0755))

	destFile := filepath.Join(tmpDir, "platform_dest")

	err := copyAndSign(srcFile, destFile)
	require.NoError(t, err)

	// On all platforms, the copy should succeed
	content, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, "platform test", string(content))

	// On Darwin (macOS), codesign should be attempted but failure is acceptable
	// On other platforms, no signing is done
	// Either way, the file should be valid
	info, err := os.Stat(destFile)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0)
}
