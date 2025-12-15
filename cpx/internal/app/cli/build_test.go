package cli

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBazelBuild(t *testing.T) {
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

	tests := []struct {
		name       string
		release    bool
		target     string
		clean      bool
		verbose    bool
		sanitizer  string
		wantConfig string
	}{
		{
			name:       "Debug build",
			release:    false,
			target:     "",
			clean:      false,
			verbose:    false,
			wantConfig: "--config=debug",
		},
		{
			name:       "Release build",
			release:    true,
			target:     "",
			clean:      false,
			verbose:    false,
			wantConfig: "--config=release",
		},
		{
			name:       "Build with target",
			release:    false,
			target:     "//src:mylib",
			clean:      false,
			verbose:    false,
			wantConfig: "--config=debug",
		},
		{
			name:       "Clean build",
			release:    false,
			target:     "",
			clean:      true,
			verbose:    false,
			wantConfig: "--config=debug",
		},
		{
			name:       "Verbose build",
			release:    false,
			target:     "",
			clean:      false,
			verbose:    true,
			wantConfig: "--config=debug",
		},
		{
			name:       "ASan build",
			release:    false,
			target:     "",
			clean:      false,
			verbose:    false,
			sanitizer:  "asan",
			wantConfig: "--config=debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedArgs = nil
			err := runBazelBuild(tt.release, tt.target, tt.clean, tt.verbose, "", tt.sanitizer)
			assert.NoError(t, err)

			// Check that bazel build was called
			foundBuild := false
			for _, args := range capturedArgs {
				if len(args) >= 2 && args[0] == "bazel" && args[1] == "build" {
					foundBuild = true
					assert.Contains(t, args, tt.wantConfig)
					if tt.target != "" {
						assert.Contains(t, args, tt.target)
					}
					if tt.verbose {
						// When verbose, --noshow_progress should NOT be added
						assert.NotContains(t, args, "--noshow_progress")
					} else {
						// When not verbose, --noshow_progress should be added
						assert.Contains(t, args, "--noshow_progress")
					}
					if tt.sanitizer == "asan" {
						assert.Contains(t, args, "--copt=-fsanitize=address")
						assert.Contains(t, args, "--linkopt=-fsanitize=address")
					}
					break
				}
			}
			assert.True(t, foundBuild, "bazel build command should be called")

			// If clean was requested, check for bazel clean
			if tt.clean {
				foundClean := false
				for _, args := range capturedArgs {
					if len(args) >= 2 && args[0] == "bazel" && args[1] == "clean" {
						foundClean = true
						break
					}
				}
				assert.True(t, foundClean, "bazel clean should be called with clean=true")
			}
		})
	}
}
