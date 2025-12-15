package build

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// copyAndSign copies a file and signs it on macOS to prevent signal: killed
func copyAndSign(src, dest string) error {
	// Remove destination to ensure clean copy
	os.Remove(dest)

	// Simple copy for Windows
	if runtime.GOOS == "windows" {
		input, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, input, 0755)
	}

	// Use cp -f on unix-like systems to preserve attributes
	cmd := exec.Command("cp", "-f", src, dest)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// On macOS/Darwin, force ad-hoc codesign
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("codesign", "-s", "-", "--force", dest)
		// We ignore error here because codesign might not be available or needed
		// but it fixes the ASan issue most of the time
		_ = cmd.Run()
	}
	return nil
}
