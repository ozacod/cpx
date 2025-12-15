package cli

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/ozacod/cpx/internal/app/cli/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess isn't a real test. It's used as a helper process
// for TestRunMesonAdd and others that need to mock exec.Command.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command provided\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "meson":
		if len(args) > 0 && args[0] == "wrap" && args[1] == "install" {
			pkg := args[2]
			// Simulate success
			if pkg == "spdlog" || pkg == "gtest" || pkg == "google-benchmark" {
				fmt.Printf("installed %s\n", pkg)
				os.Exit(0)
			}
			// Simulate failure
			fmt.Fprintf(os.Stderr, "Package %s not found\n", pkg)
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func TestRunMesonAdd(t *testing.T) {
	// Mock execCommand and execLookPath
	oldExecCommand := execCommand
	oldExecLookPath := execLookPath
	defer func() {
		execCommand = oldExecCommand
		execLookPath = oldExecLookPath
	}()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	execLookPath = func(file string) (string, error) {
		return "/usr/bin/meson", nil
	}

	// Create temp dir for test
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Create meson.build to be detected as meson project
	require.NoError(t, os.WriteFile("meson.build", []byte("project('test')"), 0644))

	tests := []struct {
		name      string
		args      []string
		wantError bool
		wantFile  bool // check if subprojects dir created
	}{
		{
			name:      "Install valid package",
			args:      []string{"spdlog"},
			wantError: false,
			wantFile:  true,
		},
		{
			name:      "Install invalid package",
			args:      []string{"unknown-pkg"},
			wantError: false, // cpx add shouldn't error out completely, just print error
			wantFile:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runMesonAdd(tt.args)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantFile {
				assert.DirExists(t, "subprojects")
			}
		})
	}
}

func TestDownloadMesonWrap(t *testing.T) {
	// Mock execCommand and execLookPath
	oldExecCommand := execCommand
	oldExecLookPath := execLookPath
	defer func() {
		execCommand = oldExecCommand
		execLookPath = oldExecLookPath
	}()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	execLookPath = func(file string) (string, error) {
		return "/usr/bin/meson", nil
	}

	tmpDir := t.TempDir()

	err := downloadMesonWrap(tmpDir, "gtest")
	assert.NoError(t, err)
}

func TestCreateProjectFromTUI_Meson(t *testing.T) {
	// Mock execCommand and execLookPath
	oldExecCommand := execCommand
	oldExecLookPath := execLookPath
	defer func() {
		execCommand = oldExecCommand
		execLookPath = oldExecLookPath
	}()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	execLookPath = func(file string) (string, error) {
		return "/usr/bin/meson", nil
	}

	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	config := tui.ProjectConfig{
		Name:           "meson-proj",
		PackageManager: "meson",
		CppStandard:    20,
		TestFramework:  "googletest",
		Benchmark:      "google-benchmark",
		VCS:            "git",
	}

	err = createProjectFromTUI(config, nil)
	assert.NoError(t, err)

	// Verify files created
	assert.FileExists(t, "meson-proj/meson.build")
	assert.FileExists(t, "meson-proj/src/meson.build")
	assert.FileExists(t, "meson-proj/tests/meson.build")
	assert.FileExists(t, "meson-proj/bench/meson.build")
	assert.DirExists(t, "meson-proj/subprojects")

	// Verify content (basic check)
	content, _ := os.ReadFile("meson-proj/meson.build")
	assert.Contains(t, string(content), "project('meson-proj', 'cpp'")
	assert.Contains(t, string(content), "cpp_std=c++20")
}

func TestRunMesonBuild_Args(t *testing.T) {
	// Mock execCommand and execLookPath
	oldExecCommand := execCommand
	oldExecLookPath := execLookPath
	defer func() {
		execCommand = oldExecCommand
		execLookPath = oldExecLookPath
	}()

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

	// Test Debug Build
	capturedArgs = nil
	err = runMesonBuild(false, "", false, false, "", "") // release=false
	assert.NoError(t, err)

	require.Len(t, capturedArgs, 3) // setup, compile, copy
	// meson setup
	assert.Equal(t, "meson", capturedArgs[0][0])
	assert.Equal(t, "setup", capturedArgs[0][1])
	assert.Contains(t, capturedArgs[0], "--buildtype=debug")
	// meson compile
	assert.Equal(t, "meson", capturedArgs[1][0])
	assert.Equal(t, "compile", capturedArgs[1][1])

	// Test Release Build
	// Note: builddir already exists, so setup will be SKIPPED unless we clean or use a fresh dir.
	// Let's use clean=true to force setup? No, clean=true deletes builddir.
	capturedArgs = nil
	err = runMesonBuild(true, "", true, false, "", "") // release=true, clean=true
	assert.NoError(t, err)

	// With clean=true:
	// 1. bazel clean (wait, this is meson build? runMesonBuild calls os.RemoveAll(buildDir), not exec command for clean)
	// So calls should be: setup, compile, copy
	require.Len(t, capturedArgs, 3)
	assert.Equal(t, "setup", capturedArgs[0][1])
	assert.Contains(t, capturedArgs[0], "--buildtype=release")
}
