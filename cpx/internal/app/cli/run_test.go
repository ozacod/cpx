package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBazelRun(t *testing.T) {
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
		release    bool
		target     string
		args       []string
		verbose    bool
		sanitizer  string
		wantConfig string
	}{
		{
			name:       "Debug run",
			release:    false,
			target:     "app",
			args:       nil,
			verbose:    false,
			wantConfig: "--config=debug",
		},
		{
			name:       "Release run",
			release:    true,
			target:     "app",
			args:       nil,
			verbose:    false,
			wantConfig: "--config=release",
		},
		{
			name:       "Run with args",
			release:    false,
			target:     "app",
			args:       []string{"--flag", "value"},
			verbose:    false,
			wantConfig: "--config=debug",
		},
		{
			name:       "ASan run",
			release:    false,
			target:     "app",
			args:       nil,
			verbose:    false,
			sanitizer:  "asan",
			wantConfig: "--config=debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedArgs = nil
			err := runBazelRun(tt.release, tt.target, tt.args, tt.verbose, "", tt.sanitizer)
			assert.NoError(t, err)

			require.GreaterOrEqual(t, len(capturedArgs), 1)
			// Check bazel run command
			assert.Equal(t, "bazel", capturedArgs[0][0])
			assert.Equal(t, "run", capturedArgs[0][1])
			assert.Contains(t, capturedArgs[0], tt.wantConfig)

			if tt.sanitizer == "asan" {
				assert.Contains(t, capturedArgs[0], "--copt=-fsanitize=address")
			}
		})
	}
}

func TestRunMesonRun(t *testing.T) {
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
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create meson.build
	require.NoError(t, os.WriteFile("meson.build", []byte("project('test', 'cpp')"), 0644))

	// Create builddir/src with executable
	srcDir := filepath.Join("builddir", "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "myapp"), []byte("#!/bin/sh\necho hello"), 0755))

	err = runMesonRun(false, "myapp", nil, false, "", "")
	// Will fail because the mock doesn't actually run meson setup correctly,
	// but we're testing that the function runs without panic
	// The actual meson setup calls are mocked
	assert.NoError(t, err)
}

func TestFindBazelMainTarget(t *testing.T) {
	// Use temp dir
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	tests := []struct {
		name         string
		buildContent string
		wantTarget   string
		wantError    bool
	}{
		{
			name: "Find main target",
			buildContent: `cc_binary(
    name = "myapp",
    srcs = ["main.cc"],
)`,
			wantTarget: "//:myapp",
			wantError:  false,
		},
		{
			name: "Skip library targets",
			buildContent: `cc_library(
    name = "myapp_lib",
    srcs = ["lib.cc"],
)

cc_binary(
    name = "main",
    srcs = ["main.cc"],
)`,
			wantTarget: "//:main",
			wantError:  false,
		},
		{
			name:         "No BUILD.bazel file",
			buildContent: "",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			os.Remove("BUILD.bazel")

			if tt.buildContent != "" {
				require.NoError(t, os.WriteFile("BUILD.bazel", []byte(tt.buildContent), 0644))
			}

			target, err := findBazelMainTarget()

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantTarget, target)
			}
		})
	}
}
