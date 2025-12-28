package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GetGitTrackedCppFiles() ([]string, error) {
	// Check if we're in a git repository
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git not found")
	}

	// Check if current directory is a git repo
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("not in a git repository")
	}

	// Get all git-tracked files
	cmd = exec.Command("git", "ls-files")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git tracked files: %w", err)
	}

	allTrackedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")

	// C/C++ file extensions
	cppExtensions := map[string]bool{
		".cpp": true, ".cxx": true, ".cc": true, ".c++": true,
		".hpp": true, ".hxx": true, ".hh": true, ".h++": true,
		".c": true, ".h": true,
		".cppm": true, ".ixx": true, // C++20 modules
	}

	// Filter to only C/C++ files
	var trackedCppFiles []string
	for _, file := range allTrackedFiles {
		if file == "" {
			continue
		}
		ext := filepath.Ext(file)
		if cppExtensions[ext] {
			// Check if file exists (git ls-files includes deleted files)
			if _, err := os.Stat(file); err == nil {
				trackedCppFiles = append(trackedCppFiles, file)
			}
		}
	}

	return trackedCppFiles, nil
}

func FilterGitTrackedFiles(targets []string) ([]string, error) {
	// Check if we're in a git repository
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git not found")
	}

	// Check if current directory is a git repo
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("not in a git repository")
	}

	// Get all git-tracked files
	cmd = exec.Command("git", "ls-files")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git tracked files: %w", err)
	}

	allTrackedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")

	// C/C++ file extensions
	cppExtensions := map[string]bool{
		".cpp": true, ".cxx": true, ".cc": true, ".c++": true,
		".hpp": true, ".hxx": true, ".hh": true, ".h++": true,
		".c": true, ".h": true,
		".cppm": true, ".ixx": true, // C++20 modules
	}

	// Filter to only C/C++ files
	var trackedCppFiles []string
	for _, file := range allTrackedFiles {
		if file == "" {
			continue
		}
		ext := filepath.Ext(file)
		if cppExtensions[ext] {
			// Check if file exists (git ls-files includes deleted files)
			if _, err := os.Stat(file); err == nil {
				trackedCppFiles = append(trackedCppFiles, file)
			}
		}
	}

	// If targets are specified, filter to only files within those targets
	if len(targets) > 0 && targets[0] != "." {
		var filtered []string
		for _, target := range targets {
			// Convert target to absolute path for comparison
			absTarget, err := filepath.Abs(target)
			if err != nil {
				continue
			}

			for _, file := range trackedCppFiles {
				absFile, err := filepath.Abs(file)
				if err != nil {
					continue
				}
				// Check if file is within the target directory
				if strings.HasPrefix(absFile, absTarget) || absFile == absTarget {
					filtered = append(filtered, file)
				}
			}
		}
		return filtered, nil
	}

	return trackedCppFiles, nil
}
