package cli

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBazelTest(t *testing.T) {
	// Mock execCommand
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	var capturedArgs [][]string

	execCommand = func(name string, arg ...string) *exec.Cmd {
		args := append([]string{name}, arg...)
		capturedArgs = append(capturedArgs, args)

		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	tests := []struct {
		name       string
		verbose    bool
		filter     string
		wantOutput string
	}{
		{
			name:       "Run all tests quietly",
			verbose:    false,
			filter:     "",
			wantOutput: "--test_output=errors",
		},
		{
			name:       "Run all tests verbose",
			verbose:    true,
			filter:     "",
			wantOutput: "--test_output=all",
		},
		{
			name:       "Run filtered tests",
			verbose:    false,
			filter:     "//tests:unit_test",
			wantOutput: "//tests:unit_test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedArgs = nil
			err := runBazelTest(tt.verbose, tt.filter)
			assert.NoError(t, err)

			require.GreaterOrEqual(t, len(capturedArgs), 1)
			// Check bazel test command
			assert.Equal(t, "bazel", capturedArgs[0][0])
			assert.Equal(t, "test", capturedArgs[0][1])
			assert.Contains(t, capturedArgs[0], tt.wantOutput)
		})
	}
}

func TestRunMesonTest(t *testing.T) {
	// Mock execCommand
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	var capturedArgs [][]string

	execCommand = func(name string, arg ...string) *exec.Cmd {
		args := append([]string{name}, arg...)
		capturedArgs = append(capturedArgs, args)

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
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create meson.build and builddir
	require.NoError(t, os.WriteFile("meson.build", []byte("project('test', 'cpp')"), 0644))
	require.NoError(t, os.MkdirAll("builddir", 0755))

	tests := []struct {
		name    string
		verbose bool
		filter  string
	}{
		{
			name:    "Run all tests quietly",
			verbose: false,
			filter:  "",
		},
		{
			name:    "Run all tests verbose",
			verbose: true,
			filter:  "",
		},
		{
			name:    "Run filtered tests",
			verbose: false,
			filter:  "mytest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedArgs = nil
			err := runMesonTest(tt.verbose, tt.filter)
			assert.NoError(t, err)

			require.GreaterOrEqual(t, len(capturedArgs), 1)
			// Check meson test command was called
			found := false
			for _, args := range capturedArgs {
				if len(args) >= 2 && args[0] == "meson" && args[1] == "test" {
					found = true
					if tt.verbose {
						assert.Contains(t, args, "-v")
					} else {
						assert.Contains(t, args, "--quiet")
					}
					break
				}
			}
			assert.True(t, found, "meson test command should be called")
		})
	}
}
