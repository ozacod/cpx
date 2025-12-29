package meson

import (
	"os"
	"path/filepath"
	"regexp"
)

func GetProjectNameFromMesonBuild(projectRoot string) string {
	mesonBuildPath := filepath.Join(projectRoot, "meson.build")
	data, err := os.ReadFile(mesonBuildPath)
	if err != nil {
		return ""
	}

	// Look for: project('name', ...) or project("name", ...)
	re := regexp.MustCompile(`project\s*\(\s*['"]([^'"]+)['"]`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}
