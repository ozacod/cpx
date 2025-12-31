package bazel

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/ozacod/cpx/internal/build/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess isn't a real test. It's used as a helper process
// for mocking exec.Command.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if out := os.Getenv("MOCK_OUTPUT"); out != "" {
		fmt.Print(out)
	}
	os.Exit(0)
}

func TestBuild(t *testing.T) {
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

	builder := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedArgs = nil

			opts := build.BuildOptions{
				Release:   tt.release,
				Target:    tt.target,
				Clean:     tt.clean,
				Verbose:   tt.verbose,
				Sanitizer: tt.sanitizer,
			}

			err := builder.Build(opts)
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
			// In Builder.Clean (called by Build), it calls bazel clean.
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

func TestRun(t *testing.T) {
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

	// Create BUILD.bazel
	require.NoError(t, os.WriteFile("BUILD.bazel", []byte(`
cc_binary(
    name = "main",
    srcs = ["main.cc"],
)
`), 0644))

	builder := New()

	err = builder.Run(build.RunOptions{
		Release: false,
		Target:  "//:main",
		Verbose: true,
	})
	assert.NoError(t, err)

	require.Len(t, capturedArgs, 1) // bazel run
	assert.Equal(t, "bazel", capturedArgs[0][0])
	assert.Equal(t, "run", capturedArgs[0][1])
	assert.Contains(t, capturedArgs[0], "//:main")
}

func TestTest(t *testing.T) {
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

	builder := New()

	err := builder.Test(build.TestOptions{
		Verbose: true,
		Filter:  "//:main_test",
	})
	assert.NoError(t, err)

	require.Len(t, capturedArgs, 1) // bazel test
	assert.Equal(t, "bazel", capturedArgs[0][0])
	assert.Equal(t, "test", capturedArgs[0][1])
	assert.Contains(t, capturedArgs[0], "//:main_test")
}

func TestBench(t *testing.T) {
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

	builder := New()

	// Test Bench with specific target
	err := builder.Bench(build.BenchOptions{
		Verbose: true,
		Target:  "//bench:myapp_bench",
	})
	assert.NoError(t, err)

	require.Len(t, capturedArgs, 1) // bazel run //bench:myapp_bench
	assert.Equal(t, "bazel", capturedArgs[0][0])
	assert.Equal(t, "run", capturedArgs[0][1])
	assert.Contains(t, capturedArgs[0], "//bench:myapp_bench")
}

func TestClean(t *testing.T) {
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

	// Create Bazel project files and artifacts
	require.NoError(t, os.WriteFile("MODULE.bazel", []byte("module(name = \"test\")"), 0644))
	directories := []string{"build", ".bin", ".out", "bazel-bin", "bazel-out", ".bazel", "external"}

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
			for _, dir := range directories {
				_ = os.MkdirAll(dir, 0755)
			}

			builder := New()
			err = builder.Clean(build.CleanOptions{All: tt.all})
			assert.NoError(t, err)

			// Check that bazel clean was called
			assert.GreaterOrEqual(t, len(capturedArgs), 1)
			assert.Equal(t, "bazel", capturedArgs[len(capturedArgs)-1][0])
			assert.Equal(t, "clean", capturedArgs[len(capturedArgs)-1][1])

			// Verify build directory was removed
			_, err = os.Stat("build")
			assert.True(t, os.IsNotExist(err), "build directory should be removed")

			// Verify .bin was removed
			_, err = os.Stat(".bin")
			assert.True(t, os.IsNotExist(err), ".bin should be removed")

			// Verify symlinks/dirs were removed/handled
			// The Bazel builder implementation tries to remove bazel-* symlinks relying on glob/stat

			if tt.all {
				_, err = os.Stat(".bazel")
				assert.True(t, os.IsNotExist(err), ".bazel should be removed with --all")
			}
		})
	}
}

func TestAddDependency(t *testing.T) {
	// Create a temporary directory for MODULE.bazel
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create MODULE.bazel file
	moduleContent := `module(name = "test")

bazel_dep(name = "rules_cc", version = "0.0.1")
`
	require.NoError(t, os.WriteFile("MODULE.bazel", []byte(moduleContent), 0644))

	// Test adding a dependency with explicit version (no BCR needed)
	builder := New()

	// Capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Add with explicit version (BCR not needed)
	err = builder.AddDependency("com_google_googletest", "1.14.0")

	// Restore stdout
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close pipe: %v", err)
	}
	os.Stdout = old

	// Read captured output
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	output := buf.String()

	// Verify
	assert.NoError(t, err)
	assert.Contains(t, output, "com_google_googletest")
	assert.Contains(t, output, "1.14.0")

	// Verify MODULE.bazel was updated
	content, err := os.ReadFile("MODULE.bazel")
	require.NoError(t, err)
	assert.Contains(t, string(content), "com_google_googletest")
	assert.Contains(t, string(content), "1.14.0")
}

func TestRemoveDependency(t *testing.T) {
	// Create a temporary directory for MODULE.bazel
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create MODULE.bazel file with a dependency
	moduleContent := `module(name = "test")

bazel_dep(name = "rules_cc", version = "0.0.1")
bazel_dep(name = "com_google_googletest", version = "1.14.0")
`
	require.NoError(t, os.WriteFile("MODULE.bazel", []byte(moduleContent), 0644))

	builder := New()

	// Remove the dependency
	err = builder.RemoveDependency("com_google_googletest")
	assert.NoError(t, err)

	// Verify MODULE.bazel was updated
	content, err := os.ReadFile("MODULE.bazel")
	require.NoError(t, err)
	assert.NotContains(t, string(content), "com_google_googletest")
	assert.Contains(t, string(content), "rules_cc") // Other deps should remain
}

func TestListDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	content := `module(name = "test")
bazel_dep(name = "zlib", version = "1.2.13")
bazel_dep(name = "gtest", version = "1.11.0")`
	require.NoError(t, os.WriteFile("MODULE.bazel", []byte(content), 0644))

	builder := New()
	deps, err := builder.ListDependencies()
	assert.NoError(t, err)
	assert.Len(t, deps, 2)
	assert.Equal(t, "zlib", deps[0].Name)
	assert.Equal(t, "1.2.13", deps[0].Version)
}

func TestName(t *testing.T) {
	builder := New()
	assert.Equal(t, "bazel", builder.Name())
}

func TestListTargets(t *testing.T) {
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

		if name == "bazel" && len(arg) > 0 && arg[0] == "query" {
			cmd.Env = append(cmd.Env, "MOCK_OUTPUT=cc_binary rule //src:main\ncc_library rule //src:mylib")
		}
		return cmd
	}

	builder := New()
	targets, err := builder.ListTargets()
	assert.NoError(t, err)
	assert.Contains(t, targets, "//src:main (cc_binary)")
	assert.Contains(t, targets, "//src:mylib (cc_library)")
}
