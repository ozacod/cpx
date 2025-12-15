package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBazelBench(t *testing.T) {
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

	// Create bench/BUILD.bazel with a target
	require.NoError(t, os.MkdirAll("bench", 0755))
	buildContent := `cc_binary(
    name = "myapp_bench",
    srcs = ["bench.cc"],
)`
	require.NoError(t, os.WriteFile("bench/BUILD.bazel", []byte(buildContent), 0644))

	tests := []struct {
		name    string
		verbose bool
		target  string
	}{
		{
			name:    "Run bench with target",
			verbose: false,
			target:  "//bench:myapp_bench",
		},
		{
			name:    "Run bench verbose",
			verbose: true,
			target:  "//bench:myapp_bench",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedArgs = nil
			err := runBazelBench(tt.verbose, tt.target)
			assert.NoError(t, err)

			require.GreaterOrEqual(t, len(capturedArgs), 1)
			// Check bazel run command was called
			found := false
			for _, args := range capturedArgs {
				if len(args) >= 2 && args[0] == "bazel" && args[1] == "run" {
					found = true
					if tt.verbose {
						assert.Contains(t, args, "--verbose_failures")
					} else {
						assert.Contains(t, args, "--noshow_progress")
					}
					break
				}
			}
			assert.True(t, found, "bazel run command should be called")
		})
	}
}

func TestRunMesonBench(t *testing.T) {
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

	// Create meson.build and builddir
	require.NoError(t, os.WriteFile("meson.build", []byte("project('test', 'cpp')"), 0644))
	benchDir := filepath.Join("builddir", "bench")
	require.NoError(t, os.MkdirAll(benchDir, 0755))

	// Create a bench executable
	benchExe := filepath.Join(benchDir, "myapp_bench")
	require.NoError(t, os.WriteFile(benchExe, []byte("#!/bin/sh\necho bench"), 0755))

	err = runMesonBench(false, "")
	assert.NoError(t, err)

	// Test with specific target
	err = runMesonBench(false, "myapp_bench")
	assert.NoError(t, err)
}

func TestFindBenchTarget(t *testing.T) {
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
	}{
		{
			name: "Find bench target",
			buildContent: `cc_binary(
    name = "myapp_bench",
    srcs = ["bench.cc"],
)`,
			wantTarget: "//bench:myapp_bench",
		},
		{
			name:         "No BUILD.bazel file",
			buildContent: "",
			wantTarget:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			os.RemoveAll("bench")

			if tt.buildContent != "" {
				require.NoError(t, os.MkdirAll("bench", 0755))
				require.NoError(t, os.WriteFile("bench/BUILD.bazel", []byte(tt.buildContent), 0644))
			}

			target := findBenchTarget()
			assert.Equal(t, tt.wantTarget, target)
		})
	}
}
