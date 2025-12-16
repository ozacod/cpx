package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteHook(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, "pre-commit")
	content := "#!/bin/bash\necho 'test'"

	err := writeHook(hookPath, content)
	require.NoError(t, err)

	// Verify file was created
	data, err := os.ReadFile(hookPath)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))

	// Verify file is executable
	info, err := os.Stat(hookPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
}

func TestWriteHook_RemovesSampleFile(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, "pre-commit")
	samplePath := hookPath + ".sample"

	// Create a sample file
	require.NoError(t, os.WriteFile(samplePath, []byte("sample"), 0644))

	content := "#!/bin/bash\necho 'test'"
	err := writeHook(hookPath, content)
	require.NoError(t, err)

	// Verify sample file was removed
	_, err = os.Stat(samplePath)
	assert.True(t, os.IsNotExist(err))

	// Verify hook was created
	_, err = os.Stat(hookPath)
	assert.NoError(t, err)
}

func TestInstallPreCommitHook(t *testing.T) {
	tests := []struct {
		name          string
		checks        []string
		expectedParts []string
		notExpected   []string
	}{
		{
			name:   "Default checks (fmt, lint)",
			checks: []string{},
			expectedParts: []string{
				"#!/bin/bash",
				"# Cpx pre-commit hook",
				"cpx fmt",
				"cpx lint",
			},
			notExpected: []string{
				"cpx test",
				"cpx flawfinder",
			},
		},
		{
			name:   "fmt only",
			checks: []string{"fmt"},
			expectedParts: []string{
				"cpx fmt",
				"Formatting code",
			},
			notExpected: []string{
				"cpx lint",
				"cpx test",
			},
		},
		{
			name:   "lint only",
			checks: []string{"lint"},
			expectedParts: []string{
				"cpx lint",
				"Running linter",
			},
		},
		{
			name:   "test check",
			checks: []string{"test"},
			expectedParts: []string{
				"cpx test",
				"Running tests",
				"exit 1", // test failures should abort commit
			},
		},
		{
			name:   "flawfinder check",
			checks: []string{"flawfinder"},
			expectedParts: []string{
				"cpx flawfinder",
				"Running Flawfinder",
			},
		},
		{
			name:   "cppcheck check",
			checks: []string{"cppcheck"},
			expectedParts: []string{
				"cpx cppcheck",
				"Running Cppcheck",
			},
		},
		{
			name:   "check command",
			checks: []string{"check"},
			expectedParts: []string{
				"cpx check",
				"Running code check",
			},
		},
		{
			name:   "multiple checks",
			checks: []string{"fmt", "lint", "test"},
			expectedParts: []string{
				"cpx fmt",
				"cpx lint",
				"cpx test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			err := InstallPreCommitHook(tmpDir, tt.checks)
			require.NoError(t, err)

			hookPath := filepath.Join(tmpDir, "pre-commit")
			content, err := os.ReadFile(hookPath)
			require.NoError(t, err)

			contentStr := string(content)
			for _, part := range tt.expectedParts {
				assert.Contains(t, contentStr, part, "Hook should contain: %s", part)
			}
			for _, part := range tt.notExpected {
				assert.NotContains(t, contentStr, part, "Hook should not contain: %s", part)
			}
		})
	}
}

func TestInstallPrePushHook(t *testing.T) {
	tests := []struct {
		name          string
		checks        []string
		expectedParts []string
		notExpected   []string
	}{
		{
			name:   "Default checks (test)",
			checks: []string{},
			expectedParts: []string{
				"#!/bin/bash",
				"# Cpx pre-push hook",
				"cpx test",
				"Push aborted",
			},
		},
		{
			name:   "test only",
			checks: []string{"test"},
			expectedParts: []string{
				"cpx test",
				"Running tests",
				"exit 1", // test failures should abort push
			},
		},
		{
			name:   "lint check",
			checks: []string{"lint"},
			expectedParts: []string{
				"cpx lint",
				"Running linter",
			},
		},
		{
			name:   "multiple checks",
			checks: []string{"test", "lint", "flawfinder"},
			expectedParts: []string{
				"cpx test",
				"cpx lint",
				"cpx flawfinder",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			err := InstallPrePushHook(tmpDir, tt.checks)
			require.NoError(t, err)

			hookPath := filepath.Join(tmpDir, "pre-push")
			content, err := os.ReadFile(hookPath)
			require.NoError(t, err)

			contentStr := string(content)
			for _, part := range tt.expectedParts {
				assert.Contains(t, contentStr, part, "Hook should contain: %s", part)
			}
			for _, part := range tt.notExpected {
				assert.NotContains(t, contentStr, part, "Hook should not contain: %s", part)
			}
		})
	}
}

func TestInstallHooksWithConfig(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	cmd := exec.Command("git", "init")
	require.NoError(t, cmd.Run())

	tests := []struct {
		name            string
		preCommit       []string
		prePush         []string
		expectPreCommit bool
		expectPrePush   bool
	}{
		{
			name:            "Both hooks",
			preCommit:       []string{"fmt", "lint"},
			prePush:         []string{"test"},
			expectPreCommit: true,
			expectPrePush:   true,
		},
		{
			name:            "Pre-commit only",
			preCommit:       []string{"fmt"},
			prePush:         []string{},
			expectPreCommit: true,
			expectPrePush:   false,
		},
		{
			name:            "Pre-push only",
			preCommit:       []string{},
			prePush:         []string{"test"},
			expectPreCommit: false,
			expectPrePush:   true,
		},
		{
			name:            "No hooks",
			preCommit:       []string{},
			prePush:         []string{},
			expectPreCommit: false,
			expectPrePush:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get git directory
			cmd := exec.Command("git", "rev-parse", "--git-dir")
			output, err := cmd.Output()
			require.NoError(t, err)
			gitDir := strings.TrimSpace(string(output))
			hooksDir := filepath.Join(gitDir, "hooks")

			// Clean up hooks before test
			os.Remove(filepath.Join(hooksDir, "pre-commit"))
			os.Remove(filepath.Join(hooksDir, "pre-push"))

			err = InstallHooksWithConfig(tt.preCommit, tt.prePush)
			require.NoError(t, err)

			// Check pre-commit hook
			preCommitPath := filepath.Join(hooksDir, "pre-commit")
			_, err = os.Stat(preCommitPath)
			if tt.expectPreCommit {
				assert.NoError(t, err, "pre-commit hook should exist")
			} else {
				assert.True(t, os.IsNotExist(err), "pre-commit hook should not exist")
			}

			// Check pre-push hook
			prePushPath := filepath.Join(hooksDir, "pre-push")
			_, err = os.Stat(prePushPath)
			if tt.expectPrePush {
				assert.NoError(t, err, "pre-push hook should exist")
			} else {
				assert.True(t, os.IsNotExist(err), "pre-push hook should not exist")
			}
		})
	}
}

func TestInstallHooksWithConfig_NotInGitRepo(t *testing.T) {
	// Create a temporary directory without a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	err = InstallHooksWithConfig([]string{"fmt"}, []string{"test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in a git repository")
}

func TestInstallPreCommitHook_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with mixed case
	err := InstallPreCommitHook(tmpDir, []string{"FMT", "Lint", "TEST"})
	require.NoError(t, err)

	hookPath := filepath.Join(tmpDir, "pre-commit")
	content, err := os.ReadFile(hookPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "cpx fmt")
	assert.Contains(t, contentStr, "cpx lint")
	assert.Contains(t, contentStr, "cpx test")
}

func TestInstallPreCommitHook_TrimWhitespace(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with whitespace around check names
	err := InstallPreCommitHook(tmpDir, []string{"  fmt  ", "\tlint\t"})
	require.NoError(t, err)

	hookPath := filepath.Join(tmpDir, "pre-commit")
	content, err := os.ReadFile(hookPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "cpx fmt")
	assert.Contains(t, contentStr, "cpx lint")
}

func TestInstallHooksWithConfig_CreatesHooksDir(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	cmd := exec.Command("git", "init")
	require.NoError(t, cmd.Run())

	// Remove hooks directory to test creation
	gitDir := filepath.Join(tmpDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	os.RemoveAll(hooksDir)

	// Verify hooks dir doesn't exist
	_, err = os.Stat(hooksDir)
	require.True(t, os.IsNotExist(err))

	// Install hooks
	err = InstallHooksWithConfig([]string{"fmt"}, []string{})
	require.NoError(t, err)

	// Verify hooks dir was created
	info, err := os.Stat(hooksDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestInstallHooksWithConfig_RemovesSampleFiles(t *testing.T) {
	// Create a temporary directory with a git repo
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(oldWd) }()
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize git repo
	cmd := exec.Command("git", "init")
	require.NoError(t, cmd.Run())

	// Get hooks directory
	gitDir := filepath.Join(tmpDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Create sample files
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre-commit.sample"), []byte("sample"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre-push.sample"), []byte("sample"), 0644))

	// Install hooks
	err = InstallHooksWithConfig([]string{"fmt"}, []string{"test"})
	require.NoError(t, err)

	// Verify sample files were removed
	_, err = os.Stat(filepath.Join(hooksDir, "pre-commit.sample"))
	assert.True(t, os.IsNotExist(err), "pre-commit.sample should be removed")

	_, err = os.Stat(filepath.Join(hooksDir, "pre-push.sample"))
	assert.True(t, os.IsNotExist(err), "pre-push.sample should be removed")
}

func TestHookContent_ContainsCpxCheck(t *testing.T) {
	tmpDir := t.TempDir()

	err := InstallPreCommitHook(tmpDir, []string{"fmt"})
	require.NoError(t, err)

	hookPath := filepath.Join(tmpDir, "pre-commit")
	content, err := os.ReadFile(hookPath)
	require.NoError(t, err)

	// Verify hook checks for cpx command
	contentStr := string(content)
	assert.Contains(t, contentStr, "command -v cpx")
	assert.Contains(t, contentStr, "cpx not found")
}

func TestHookContent_EndsWithExit0(t *testing.T) {
	tmpDir := t.TempDir()

	// Test pre-commit hook
	err := InstallPreCommitHook(tmpDir, []string{"fmt"})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "pre-commit"))
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(strings.TrimSpace(string(content)), "exit 0"))

	// Test pre-push hook
	err = InstallPrePushHook(tmpDir, []string{"test"})
	require.NoError(t, err)

	content, err = os.ReadFile(filepath.Join(tmpDir, "pre-push"))
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(strings.TrimSpace(string(content)), "exit 0"))
}
