package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintError(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	PrintError("test error %s", "message")

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "test error message")
	assert.Contains(t, output, IconError)
}

func TestCheckCommandExists(t *testing.T) {
	// Save and restore the original execLookPath
	oldExecLookPath := execLookPath
	defer func() { execLookPath = oldExecLookPath }()

	tests := []struct {
		name     string
		command  string
		mockErr  error
		expected bool
	}{
		{
			name:     "Command exists",
			command:  "go",
			mockErr:  nil,
			expected: true,
		},
		{
			name:     "Command not found",
			command:  "nonexistent_command_12345",
			mockErr:  exec.ErrNotFound,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execLookPath = func(file string) (string, error) {
				if tt.mockErr != nil {
					return "", tt.mockErr
				}
				return "/usr/bin/" + file, nil
			}

			result := CheckCommandExists(tt.command)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "File exists",
			path:     testFile,
			expected: true,
		},
		{
			name:     "File does not exist",
			path:     filepath.Join(tmpDir, "nonexistent.txt"),
			expected: false,
		},
		{
			name:     "Directory exists",
			path:     tmpDir,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckFileExists(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSpinner(t *testing.T) {
	// Test Spinner.Tick
	t.Run("Tick advances frame", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		s := &Spinner{
			frames:  []string{"a", "b", "c"},
			current: 0,
			message: "test",
		}

		s.Tick()
		assert.Equal(t, 1, s.current)

		s.Tick()
		assert.Equal(t, 2, s.current)

		s.Tick()
		assert.Equal(t, 0, s.current) // Should wrap around

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.Contains(t, output, "test")
	})

	// Test Spinner.Done
	t.Run("Done prints success", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		s := &Spinner{
			frames:  []string{"-"},
			current: 0,
			message: "loading",
		}

		s.Done("completed!")

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.Contains(t, output, "completed!")
		assert.Contains(t, output, IconSuccess)
	})

	// Test Spinner.Fail
	t.Run("Fail prints error", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		s := &Spinner{
			frames:  []string{"-"},
			current: 0,
			message: "loading",
		}

		s.Fail("failed!")

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.Contains(t, output, "failed!")
		assert.Contains(t, output, IconError)
	})
}

func TestCheckBuildToolsForProject(t *testing.T) {
	// Save and restore the original execLookPath
	oldExecLookPath := execLookPath
	defer func() { execLookPath = oldExecLookPath }()

	tests := []struct {
		name         string
		projectType  ProjectType
		availableCmd map[string]bool
		expectedMsgs []string
	}{
		{
			name:        "Bazel project with bazel available",
			projectType: ProjectTypeBazel,
			availableCmd: map[string]bool{
				"bazel": true,
			},
			expectedMsgs: []string{}, // No missing tools
		},
		{
			name:        "Bazel project with bazelisk available",
			projectType: ProjectTypeBazel,
			availableCmd: map[string]bool{
				"bazelisk": true,
			},
			expectedMsgs: []string{}, // No missing tools
		},
		{
			name:         "Bazel project with no bazel",
			projectType:  ProjectTypeBazel,
			availableCmd: map[string]bool{},
			expectedMsgs: []string{"bazel or bazelisk"},
		},
		{
			name:        "Meson project with all tools",
			projectType: ProjectTypeMeson,
			availableCmd: map[string]bool{
				"meson": true,
				"ninja": true,
				"gcc":   true,
				"g++":   true,
			},
			expectedMsgs: []string{},
		},
		{
			name:        "Meson project missing meson",
			projectType: ProjectTypeMeson,
			availableCmd: map[string]bool{
				"ninja": true,
				"gcc":   true,
				"g++":   true,
			},
			expectedMsgs: []string{"meson"},
		},
		{
			name:        "Meson project missing ninja",
			projectType: ProjectTypeMeson,
			availableCmd: map[string]bool{
				"meson": true,
				"gcc":   true,
				"g++":   true,
			},
			expectedMsgs: []string{"ninja"},
		},
		{
			name:        "Unknown project missing cmake",
			projectType: ProjectTypeUnknown,
			availableCmd: map[string]bool{
				"make": true,
				"gcc":  true,
				"g++":  true,
			},
			expectedMsgs: []string{"cmake"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execLookPath = func(file string) (string, error) {
				if tt.availableCmd[file] {
					return "/usr/bin/" + file, nil
				}
				return "", exec.ErrNotFound
			}

			missing := CheckBuildToolsForProject(tt.projectType)

			if len(tt.expectedMsgs) == 0 {
				assert.Empty(t, missing)
			} else {
				for _, msg := range tt.expectedMsgs {
					found := false
					for _, m := range missing {
						if contains(m, msg) {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected missing tool message containing: %s", msg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestWarnMissingBuildTools(t *testing.T) {
	// Save and restore the original execLookPath
	oldExecLookPath := execLookPath
	defer func() { execLookPath = oldExecLookPath }()

	t.Run("Prints warning when tools missing", func(t *testing.T) {
		execLookPath = func(file string) (string, error) {
			return "", exec.ErrNotFound
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		missing := WarnMissingBuildTools(ProjectTypeBazel)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.NotEmpty(t, missing)
		assert.Contains(t, output, "Warning")
	})

	t.Run("No warning when all tools present", func(t *testing.T) {
		execLookPath = func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		missing := WarnMissingBuildTools(ProjectTypeBazel)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		assert.Empty(t, missing)
		assert.NotContains(t, output, "Warning")
	})
}

func TestProjectTypeConstants(t *testing.T) {
	assert.Equal(t, ProjectType("vcpkg"), ProjectTypeVcpkg)
	assert.Equal(t, ProjectType("bazel"), ProjectTypeBazel)
	assert.Equal(t, ProjectType("meson"), ProjectTypeMeson)
	assert.Equal(t, ProjectType("unknown"), ProjectTypeUnknown)
}

func TestColorConstants(t *testing.T) {
	// Test that color constants are defined
	assert.NotEmpty(t, Reset)
	assert.NotEmpty(t, Red)
	assert.NotEmpty(t, Green)
	assert.NotEmpty(t, Yellow)
	assert.NotEmpty(t, Cyan)
	assert.NotEmpty(t, Bold)
	assert.NotEmpty(t, Dim)

	// Test ANSI escape sequence format
	assert.Equal(t, "\033[0m", Reset)
	assert.Equal(t, "\033[31m", Red)
	assert.Equal(t, "\033[32m", Green)
	assert.Equal(t, "\033[33m", Yellow)
	assert.Equal(t, "\033[36m", Cyan)
	assert.Equal(t, "\033[1m", Bold)
	assert.Equal(t, "\033[2m", Dim)
}

func TestIconConstants(t *testing.T) {
	assert.Equal(t, "✓", IconSuccess)
	assert.Equal(t, "✗", IconError)
}

func TestVersionConstant(t *testing.T) {
	assert.NotEmpty(t, Version)
	// Version should be in semver format
	assert.Regexp(t, `^\d+\.\d+\.\d+`, Version)
}

func TestDefaultServerConstant(t *testing.T) {
	assert.NotEmpty(t, DefaultServer)
	assert.Contains(t, DefaultServer, "https://")
}

func TestCheckBuildToolsForVcpkgProject(t *testing.T) {
	// Save original function
	oldExecLookPath := execLookPath
	defer func() { execLookPath = oldExecLookPath }()

	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	t.Run("Vcpkg project missing cmake", func(t *testing.T) {
		execLookPath = func(file string) (string, error) {
			switch file {
			case "make", "gcc", "g++":
				return "/usr/bin/" + file, nil
			default:
				return "", exec.ErrNotFound
			}
		}

		missing := CheckBuildToolsForProject(ProjectTypeVcpkg)

		// Should report cmake missing
		foundCmake := false
		for _, m := range missing {
			if containsSubstring(m, "cmake") {
				foundCmake = true
				break
			}
		}
		assert.True(t, foundCmake, "Should report cmake as missing")
	})

	t.Run("Vcpkg project missing compilers", func(t *testing.T) {
		execLookPath = func(file string) (string, error) {
			switch file {
			case "cmake", "make":
				return "/usr/bin/" + file, nil
			default:
				return "", exec.ErrNotFound
			}
		}

		missing := CheckBuildToolsForProject(ProjectTypeVcpkg)

		// Should report compilers missing
		foundCCompiler := false
		foundCxxCompiler := false
		for _, m := range missing {
			if containsSubstring(m, "C compiler") {
				foundCCompiler = true
			}
			if containsSubstring(m, "C++ compiler") {
				foundCxxCompiler = true
			}
		}
		assert.True(t, foundCCompiler, "Should report C compiler as missing")
		assert.True(t, foundCxxCompiler, "Should report C++ compiler as missing")
	})
}

func TestCheckBuildToolsForMesonProject_WithClang(t *testing.T) {
	oldExecLookPath := execLookPath
	defer func() { execLookPath = oldExecLookPath }()

	t.Run("Meson with clang compilers", func(t *testing.T) {
		execLookPath = func(file string) (string, error) {
			switch file {
			case "meson", "ninja", "clang", "clang++":
				return "/usr/bin/" + file, nil
			default:
				return "", exec.ErrNotFound
			}
		}

		missing := CheckBuildToolsForProject(ProjectTypeMeson)
		assert.Empty(t, missing, "Should not report missing tools when clang is available")
	})
}
