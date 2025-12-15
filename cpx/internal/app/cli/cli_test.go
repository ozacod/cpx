package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string // filename -> content
		expected ProjectType
	}{
		{
			name:     "Bazel project",
			files:    map[string]string{"MODULE.bazel": "# test bazel module"},
			expected: ProjectTypeBazel,
		},
		{
			name:     "Vcpkg project",
			files:    map[string]string{"vcpkg.json": "{}"},
			expected: ProjectTypeVcpkg,
		},
		{
			name:     "Unknown project",
			files:    nil,
			expected: ProjectTypeUnknown,
		},
		{
			name:     "Meson project",
			files:    map[string]string{"meson.build": "project('test', 'cpp')"},
			expected: ProjectTypeMeson,
		},
		{
			name:     "Vcpkg takes priority over Bazel",
			files:    map[string]string{"MODULE.bazel": "# bazel", "vcpkg.json": "{}"},
			expected: ProjectTypeVcpkg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for isolation
			tmpDir := t.TempDir()
			oldWd, err := os.Getwd()
			require.NoError(t, err)
			defer func() { _ = os.Chdir(oldWd) }()
			require.NoError(t, os.Chdir(tmpDir))

			// Create test files
			for filename, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644))
			}

			result := DetectProjectType()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRequireProject(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string]string
		cmdName      string
		expectsError bool
		expectedType ProjectType
	}{
		{
			name:         "Valid vcpkg project",
			files:        map[string]string{"vcpkg.json": "{}"},
			cmdName:      "test",
			expectsError: false,
			expectedType: ProjectTypeVcpkg,
		},
		{
			name:         "Valid bazel project",
			files:        map[string]string{"MODULE.bazel": "# test"},
			cmdName:      "build",
			expectsError: false,
			expectedType: ProjectTypeBazel,
		},
		{
			name:         "Invalid project",
			files:        nil,
			cmdName:      "build",
			expectsError: true,
			expectedType: ProjectTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for isolation
			tmpDir := t.TempDir()
			oldWd, err := os.Getwd()
			require.NoError(t, err)
			defer func() { _ = os.Chdir(oldWd) }()
			require.NoError(t, os.Chdir(tmpDir))

			// Create test files
			for filename, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644))
			}

			result, err := RequireProject(tt.cmdName)

			if tt.expectsError {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedType, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedType, result)
			}
		})
	}
}

func TestRequireVcpkgProject(t *testing.T) {
	tests := []struct {
		name         string
		files        map[string]string
		cmdName      string
		expectsError bool
	}{
		{
			name:         "Valid vcpkg project",
			files:        map[string]string{"vcpkg.json": "{}"},
			cmdName:      "test",
			expectsError: false,
		},
		{
			name:         "Invalid vcpkg project",
			files:        nil,
			cmdName:      "build",
			expectsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory for isolation
			tmpDir := t.TempDir()
			oldWd, err := os.Getwd()
			require.NoError(t, err)
			defer func() { _ = os.Chdir(oldWd) }()
			require.NoError(t, os.Chdir(tmpDir))

			// Create test files
			for filename, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644))
			}

			err = requireVcpkgProject(tt.cmdName)

			if tt.expectsError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
