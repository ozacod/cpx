package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ozacod/cpx/internal/utils/common"
)

type DockerImage struct {
	Repository   string
	Tag          string
	ID           string
	Size         string
	Created      string
	Architecture string
}

func (d DockerImage) FullName() string {
	if d.Tag == "" || d.Tag == "<none>" {
		return d.Repository
	}
	return d.Repository + ":" + d.Tag
}

type ImageCheckResult struct {
	Success bool
	Error   string
}

type ImageCheckProgress struct {
	Phase string // "pulling", "checking"
}

func listDockerImages() []DockerImage {
	cmd := exec.Command("docker", "images", "--format", "{{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}\t{{.CreatedSince}}")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var images []DockerImage
	var imageIDs []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 5 {
			if parts[0] == "<none>" {
				continue
			}
			images = append(images, DockerImage{
				Repository: parts[0],
				Tag:        parts[1],
				ID:         parts[2],
				Size:       parts[3],
				Created:    parts[4],
			})
			imageIDs = append(imageIDs, parts[2])
		}
	}

	if len(imageIDs) > 0 {
		archMap := getImageArchitectures(imageIDs)
		for i := range images {
			if arch, ok := archMap[images[i].ID]; ok {
				images[i].Architecture = arch
			}
		}
	}

	return images
}

func getImageArchitectures(imageIDs []string) map[string]string {
	archMap := make(map[string]string)

	args := append([]string{"inspect", "--format", "{{.Id}}\t{{.Architecture}}"}, imageIDs...)
	cmd := exec.Command("docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return archMap
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			// Extract short ID from full ID (sha256:xxxx...)
			fullID := parts[0]
			shortID := fullID
			if strings.HasPrefix(fullID, "sha256:") {
				shortID = fullID[7:19] // Get first 12 chars after sha256:
			}
			archMap[shortID] = parts[1]
		}
	}

	return archMap
}

func detectProjectType() string {
	if common.CheckFileExists("vcpkg.json") {
		return "vcpkg"
	}
	if common.CheckFileExists("BUILD.bazel") || common.CheckFileExists("WORKSPACE") || common.CheckFileExists("MODULE.bazel") {
		return "bazel"
	}
	if common.CheckFileExists("meson.build") {
		return "meson"
	}
	if common.CheckFileExists("CMakeLists.txt") {
		return "cmake"
	}
	return "unknown"
}

func checkDockerImageHasCommand(image, command string) bool {
	cmd := exec.Command("docker", "run", "--rm", "--entrypoint", "which", image, command)
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		return err == nil
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		return false
	}
}

func checkBuildToolsInDockerImage(image string, projectType string) []string {
	var missing []string

	switch projectType {
	case "vcpkg", "cmake":
		if !checkDockerImageHasCommand(image, "cmake") {
			missing = append(missing, "cmake")
		}
		hasMake := checkDockerImageHasCommand(image, "make")
		hasNinja := checkDockerImageHasCommand(image, "ninja")
		if !hasMake && !hasNinja {
			missing = append(missing, "make or ninja")
		}
		hasGCC := checkDockerImageHasCommand(image, "gcc")
		hasClang := checkDockerImageHasCommand(image, "clang")
		hasGPP := checkDockerImageHasCommand(image, "g++")
		hasClangPP := checkDockerImageHasCommand(image, "clang++")
		if !hasGCC && !hasClang {
			missing = append(missing, "C compiler")
		}
		if !hasGPP && !hasClangPP {
			missing = append(missing, "C++ compiler")
		}
	case "bazel":
		hasBazel := checkDockerImageHasCommand(image, "bazel")
		hasBazelisk := checkDockerImageHasCommand(image, "bazelisk")
		if !hasBazel && !hasBazelisk {
			missing = append(missing, "bazel or bazelisk")
		}
	case "meson":
		if !checkDockerImageHasCommand(image, "meson") {
			missing = append(missing, "meson")
		}
		if !checkDockerImageHasCommand(image, "ninja") {
			missing = append(missing, "ninja")
		}
		hasGCC := checkDockerImageHasCommand(image, "gcc")
		hasClang := checkDockerImageHasCommand(image, "clang")
		hasGPP := checkDockerImageHasCommand(image, "g++")
		hasClangPP := checkDockerImageHasCommand(image, "clang++")
		if !hasGCC && !hasClang {
			missing = append(missing, "C compiler")
		}
		if !hasGPP && !hasClangPP {
			missing = append(missing, "C++ compiler")
		}
	}

	return missing
}

func checkImageToolsCmd(image string) tea.Cmd {
	return func() tea.Msg {
		projectType := detectProjectType()
		if projectType != "unknown" {
			missingTools := checkBuildToolsInDockerImage(image, projectType)
			if len(missingTools) > 0 {
				return ImageCheckResult{
					Success: false,
					Error:   fmt.Sprintf("Image missing tools for %s project: %s", projectType, strings.Join(missingTools, ", ")),
				}
			}
		}
		return ImageCheckResult{Success: true}
	}
}
