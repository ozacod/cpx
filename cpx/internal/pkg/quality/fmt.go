package quality

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FormatCode formats C++ source files using clang-format
func FormatCode(checkOnly bool) error {
	// Check if clang-format is available
	if _, err := exec.LookPath("clang-format"); err != nil {
		return fmt.Errorf("clang-format not found. Please install it first")
	}

	fmt.Printf("%s Formatting code...%s\n", Cyan, Reset)

	// Find all source files
	var files []string
	extensions := []string{".cpp", ".hpp", ".c", ".h", ".cc", ".cxx", ".hxx"}

	for _, dir := range []string{"src", "include", "tests"} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			for _, ext := range extensions {
				if strings.HasSuffix(path, ext) {
					files = append(files, path)
					break
				}
			}
			return nil
		})
	}

	if len(files) == 0 {
		fmt.Printf("%s No source files found%s\n", Green, Reset)
		return nil
	}

	// Format each file
	formatArgs := []string{"-style=file"}
	if !checkOnly {
		formatArgs = append(formatArgs, "-i")
	} else {
		formatArgs = append(formatArgs, "--dry-run", "--Werror")
	}

	needsFormat := false
	for _, file := range files {
		args := append(formatArgs, file)
		cmd := exec.Command("clang-format", args...)
		output, err := cmd.CombinedOutput()

		if checkOnly && err != nil {
			needsFormat = true
			fmt.Printf("   %s %s needs formatting%s\n", Yellow, file, Reset)
		} else if !checkOnly {
			fmt.Printf("    %s\n", file)
		}

		if len(output) > 0 && checkOnly {
			fmt.Print(string(output))
		}
	}

	if checkOnly && needsFormat {
		return fmt.Errorf("some files need formatting. Run 'cpx fmt' to fix")
	}

	fmt.Printf("%s Formatted %d files%s\n", Green, len(files), Reset)
	return nil
}
