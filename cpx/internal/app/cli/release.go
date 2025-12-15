package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ozacod/cpx/internal/pkg/templates"
)

// ReleaseCmd creates the release command
func ReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Bump version number",
		Long:  "Bump version number (major, minor, or patch) in CMakeLists.txt. Defaults to patch if not specified.",
		RunE:  runRelease,
		Args:  cobra.MaximumNArgs(1),
	}

	return cmd
}

func runRelease(_ *cobra.Command, args []string) error {
	bumpType := "patch"
	if len(args) > 0 {
		bumpType = args[0]
	}
	return bumpVersion(bumpType)
}

func bumpVersion(bumpType string) error {
	// Read version from CMakeLists.txt
	cmakeContent, err := os.ReadFile("CMakeLists.txt")
	if err != nil {
		return fmt.Errorf("failed to read CMakeLists.txt: %w", err)
	}

	// Find VERSION in project() declaration
	projectRegex := regexp.MustCompile(`(?i)project\s*\(\s*([A-Za-z0-9_]+)\s+VERSION\s+(\d+\.\d+\.\d+)`)
	matches := projectRegex.FindStringSubmatch(string(cmakeContent))

	if len(matches) < 3 {
		return fmt.Errorf("could not find VERSION in CMakeLists.txt project() declaration")
	}

	projectName := matches[1]
	version := matches[2]

	// Parse version
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) < 3 {
		parts = append(parts, make([]string, 3-len(parts))...)
	}

	major, minor, patch := 0, 0, 0
	_, _ = fmt.Sscanf(parts[0], "%d", &major)
	_, _ = fmt.Sscanf(parts[1], "%d", &minor)
	_, _ = fmt.Sscanf(parts[2], "%d", &patch)

	switch bumpType {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	default:
		return fmt.Errorf("invalid bump type: %s (use major, minor, or patch)", bumpType)
	}

	newVersion := fmt.Sprintf("%d.%d.%d", major, minor, patch)

	fmt.Printf("%s Bumping version: %s → %s%s\n", Cyan, version, newVersion, Reset)

	// Replace version in CMakeLists.txt
	newContent := projectRegex.ReplaceAllStringFunc(string(cmakeContent), func(match string) string {
		subMatches := projectRegex.FindStringSubmatch(match)
		if len(subMatches) < 3 {
			return match
		}
		return strings.Replace(match, subMatches[2], newVersion, 1)
	})

	if err := os.WriteFile("CMakeLists.txt", []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write CMakeLists.txt: %w", err)
	}

	fmt.Printf("%s Version updated to %s in CMakeLists.txt%s\n", Green, newVersion, Reset)

	// Update version.hpp if it exists
	versionHeaderPath := filepath.Join("include", projectName, "version.hpp")
	if _, err := os.Stat(versionHeaderPath); err == nil {
		versionHpp := templates.GenerateVersionHpp(projectName, newVersion)
		if err := os.WriteFile(versionHeaderPath, []byte(versionHpp), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", versionHeaderPath, err)
		}
		fmt.Printf("%s Version updated to %s in %s%s\n", Green, newVersion, versionHeaderPath, Reset)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to access %s: %w", versionHeaderPath, err)
	}

	return nil
}
