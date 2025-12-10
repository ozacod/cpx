package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() error
		cleanupFunc   func() error
		expected      ProjectType
		expectsError  bool
	}{
		{
			name: "Bazel project",
			setupFunc: func() error {
				return os.WriteFile("MODULE.bazel", []byte("# test bazel module"), 0644)
			},
			cleanupFunc: func() error {
				return os.Remove("MODULE.bazel")
			},
			expected:     ProjectTypeBazel,
		},
		{
			name: "Vcpkg project",
			setupFunc: func() error {
				return os.WriteFile("vcpkg.json", []byte("{}"), 0644)
			},
			cleanupFunc: func() error {
				return os.Remove("vcpkg.json")
			},
			expected:     ProjectTypeVcpkg,
		},
		{
			name: "Unknown project",
			setupFunc: func() error {
				// No files to create
				return nil
			},
			cleanupFunc: func() error {
				// No files to clean up
				return nil
			},
			expected:     ProjectTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files
			if tt.setupFunc != nil {
				defer tt.cleanupFunc()
				require.NoError(t, tt.setupFunc())
			}

			result := DetectProjectType()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRequireProject(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func() error
		cleanupFunc  func() error
		cmdName      string
		expectsError bool
		expectedType ProjectType
	}{
		{
			name: "Valid vcpkg project",
			setupFunc: func() error {
				return os.WriteFile("vcpkg.json", []byte("{}"), 0644)
			},
			cleanupFunc: func() error {
				return os.Remove("vcpkg.json")
			},
			cmdName:      "test",
			expectsError: false,
			expectedType: ProjectTypeVcpkg,
		},
		{
			name: "Valid bazel project",
			setupFunc: func() error {
				return os.WriteFile("MODULE.bazel", []byte("# test"), 0644)
			},
			cleanupFunc: func() error {
				return os.Remove("MODULE.bazel")
			},
			cmdName:      "build",
			expectsError: false,
			expectedType: ProjectTypeBazel,
		},
		{
			name: "Invalid project",
			setupFunc: func() error {
				// No files to create
				return nil
			},
			cleanupFunc: func() error {
				// No files to clean up
				return nil
			},
			cmdName:      "build",
			expectsError: true,
			expectedType: ProjectTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files
			if tt.setupFunc != nil {
				defer tt.cleanupFunc()
				require.NoError(t, tt.setupFunc())
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
		setupFunc    func() error
		cleanupFunc  func() error
		cmdName      string
		expectsError bool
	}{
		{
			name: "Valid vcpkg project",
			setupFunc: func() error {
				return os.WriteFile("vcpkg.json", []byte("{}"), 0644)
			},
			cleanupFunc: func() error {
				return os.Remove("vcpkg.json")
			},
			cmdName:      "test",
			expectsError: false,
		},
		{
			name: "Invalid vcpkg project",
			setupFunc: func() error {
				// No files to create
				return nil
			},
			cleanupFunc: func() error {
				// No files to clean up
				return nil
			},
			cmdName:      "build",
			expectsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files
			if tt.setupFunc != nil {
				defer tt.cleanupFunc()
				require.NoError(t, tt.setupFunc())
			}

			err := requireVcpkgProject(tt.cmdName)

			if tt.expectsError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
