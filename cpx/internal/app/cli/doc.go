package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"

	"github.com/spf13/cobra"
)

// DocCmd creates the doc command
func DocCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Generate documentation",
		Long:  "Generate documentation using Doxygen. Use --open to open in browser after generation.",
		RunE:  runDoc,
	}

	cmd.Flags().Bool("open", false, "Open documentation in browser")

	return cmd
}

func runDoc(cmd *cobra.Command, _ []string) error {
	open, _ := cmd.Flags().GetBool("open")
	return generateDocs(open)
}

// getProjectInfo reads project name and version from CMakeLists.txt or vcpkg.json
func getProjectInfo() (name string, version string) {
	// Default values
	name = "Project"
	version = "0.1.0"

	// Try to read from CMakeLists.txt
	if cmakeContent, err := os.ReadFile("CMakeLists.txt"); err == nil {
		// Extract project name
		nameRegex := regexp.MustCompile(`project\s*\(\s*(\w+)`)
		if matches := nameRegex.FindStringSubmatch(string(cmakeContent)); len(matches) > 1 {
			name = matches[1]
		}
		// Extract version
		versionRegex := regexp.MustCompile(`VERSION\s+(\d+\.\d+\.\d+)`)
		if matches := versionRegex.FindStringSubmatch(string(cmakeContent)); len(matches) > 1 {
			version = matches[1]
		}
	}

	// Also try vcpkg.json as fallback for name
	if vcpkgData, err := os.ReadFile("vcpkg.json"); err == nil {
		var vcpkg map[string]interface{}
		if json.Unmarshal(vcpkgData, &vcpkg) == nil {
			if n, ok := vcpkg["name"].(string); ok && n != "" {
				name = n
			}
			if v, ok := vcpkg["version"].(string); ok && v != "" {
				version = v
			}
		}
	}

	return name, version
}

func generateDocs(openBrowser bool) error {
	// Check if Doxygen is available
	if _, err := exec.LookPath("doxygen"); err != nil {
		return fmt.Errorf("doxygen not found. Please install it first:\n  macOS: brew install doxygen\n  Ubuntu: sudo apt install doxygen")
	}

	projectName, projectVersion := getProjectInfo()

	fmt.Printf("%s Generating documentation...%s\n", Cyan, Reset)

	// Create Doxyfile if it doesn't exist
	if _, err := os.Stat("Doxyfile"); os.IsNotExist(err) {
		doxyContent := fmt.Sprintf(`PROJECT_NAME           = "%s"
PROJECT_NUMBER         = "%s"
OUTPUT_DIRECTORY       = docs
INPUT                  = src include
RECURSIVE              = YES
EXTRACT_ALL            = YES
GENERATE_HTML          = YES
GENERATE_LATEX         = NO
HTML_OUTPUT            = html
USE_MDFILE_AS_MAINPAGE = README.md
`, projectName, projectVersion)

		if err := os.WriteFile("Doxyfile", []byte(doxyContent), 0644); err != nil {
			return fmt.Errorf("failed to create Doxyfile: %w", err)
		}
		fmt.Printf("    Created Doxyfile\n")
	}

	// Run Doxygen
	cmd := exec.Command("doxygen")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("doxygen failed: %w", err)
	}

	indexPath := "docs/html/index.html"
	fmt.Printf("%s Documentation generated at %s%s\n", Green, indexPath, Reset)

	if openBrowser {
		var openCmd string
		switch runtime.GOOS {
		case "darwin":
			openCmd = "open"
		case "linux":
			openCmd = "xdg-open"
		case "windows":
			openCmd = "start"
		}

		if openCmd != "" {
			_ = exec.Command(openCmd, indexPath).Start()
		}
	}

	return nil
}
