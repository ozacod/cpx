package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WatchConfig holds configuration for file watching
type WatchConfig struct {
	Directories []string
	Extensions  []string
	IgnoreDirs  []string
	Debounce    time.Duration
}

// DefaultWatchConfig returns default watch configuration
func DefaultWatchConfig() *WatchConfig {
	return &WatchConfig{
		Directories: []string{"src", "include", "tests"},
		Extensions:  []string{".cpp", ".hpp", ".c", ".h", ".cc", ".cxx", ".hxx"},
		IgnoreDirs:  []string{"build", ".git", ".vcpkg", "vcpkg_installed", "out"},
		Debounce:    500 * time.Millisecond,
	}
}

// FileSnapshot represents a snapshot of file modification times
type FileSnapshot map[string]time.Time

// TakeSnapshot captures current modification times of watched files
func TakeSnapshot(config *WatchConfig) (FileSnapshot, error) {
	snapshot := make(FileSnapshot)

	for _, dir := range config.Directories {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip files we can't access
			}

			// Skip ignored directories
			if info.IsDir() {
				for _, ignoreDir := range config.IgnoreDirs {
					if info.Name() == ignoreDir || strings.Contains(path, string(filepath.Separator)+ignoreDir+string(filepath.Separator)) {
						return filepath.SkipDir
					}
				}
				return nil
			}

			// Check if file matches watched extensions
			ext := strings.ToLower(filepath.Ext(path))
			for _, watchExt := range config.Extensions {
				if ext == watchExt {
					snapshot[path] = info.ModTime()
					break
				}
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk directory %s: %w", dir, err)
		}
	}

	return snapshot, nil
}

// DetectChanges compares two snapshots and returns changed files
func DetectChanges(old, new FileSnapshot) []string {
	var changed []string

	// Check for new or modified files
	for path, newTime := range new {
		if oldTime, exists := old[path]; !exists || !oldTime.Equal(newTime) {
			changed = append(changed, path)
		}
	}

	// Check for deleted files
	for path := range old {
		if _, exists := new[path]; !exists {
			changed = append(changed, path+" (deleted)")
		}
	}

	return changed
}

// WatchAndBuild watches for file changes and triggers rebuilds
func WatchAndBuild(release bool, jobs int, target string, optLevel string, setupVcpkgEnv func() error) error {
	config := DefaultWatchConfig()

	fmt.Printf("\033[36m👀 Watching for changes in: %s\033[0m\n", strings.Join(config.Directories, ", "))
	fmt.Printf("\033[36m   Extensions: %s\033[0m\n", strings.Join(config.Extensions, ", "))
	fmt.Printf("\033[33m   Press Ctrl+C to stop\033[0m\n\n")

	// Initial build
	fmt.Printf("\033[36m🔨 Initial build...\033[0m\n")
	if err := BuildProject(release, jobs, target, false, optLevel, setupVcpkgEnv); err != nil {
		fmt.Printf("\033[31m✗ Build failed: %v\033[0m\n", err)
	}

	// Take initial snapshot
	snapshot, err := TakeSnapshot(config)
	if err != nil {
		return fmt.Errorf("failed to take initial snapshot: %w", err)
	}

	// Watch loop
	ticker := time.NewTicker(config.Debounce)
	defer ticker.Stop()

	for range ticker.C {
		newSnapshot, err := TakeSnapshot(config)
		if err != nil {
			fmt.Printf("\033[33m⚠ Failed to check for changes: %v\033[0m\n", err)
			continue
		}

		changes := DetectChanges(snapshot, newSnapshot)
		if len(changes) > 0 {
			fmt.Printf("\n\033[36m📝 Changes detected:\033[0m\n")
			for _, change := range changes {
				fmt.Printf("   %s\n", change)
			}
			fmt.Printf("\n\033[36m🔨 Rebuilding...\033[0m\n")

			if err := BuildProject(release, jobs, target, false, optLevel, setupVcpkgEnv); err != nil {
				fmt.Printf("\033[31m✗ Build failed: %v\033[0m\n", err)
			} else {
				fmt.Printf("\033[32m✓ Build succeeded\033[0m\n")
			}

			snapshot = newSnapshot
		}
	}

	return nil
}
