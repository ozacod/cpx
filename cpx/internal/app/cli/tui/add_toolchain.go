package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ozacod/cpx/pkg/config"
)

// ToolchainStep represents the current step in the target creation flow
type ToolchainStep int

const (
	ToolchainStepName ToolchainStep = iota
	ToolchainStepRunner
	ToolchainStepDockerMode
	ToolchainStepDockerImageSelect // New: interactive image selection
	ToolchainStepDockerImage
	ToolchainStepCheckingImage // New: async checking step
	ToolchainStepDockerfile    // for build mode
	ToolchainStepBuildContext  // for build mode
	ToolchainStepPlatform
	ToolchainStepBuildType
	ToolchainStepDone
)

// DockerImage represents a Docker image with its metadata
type DockerImage struct {
	Repository   string
	Tag          string
	ID           string
	Size         string
	Created      string
	Architecture string
}

// FullName returns the full image name (repo:tag)
func (d DockerImage) FullName() string {
	if d.Tag == "" || d.Tag == "<none>" {
		return d.Repository
	}
	return d.Repository + ":" + d.Tag
}

// ImageCheckResult is the result of async image checking
type ImageCheckResult struct {
	Success bool
	Error   string
}

// ImageCheckProgress is sent during async checking to update status
type ImageCheckProgress struct {
	Phase string // "pulling", "checking"
}

// ToolchainModel represents the TUI state for adding a CI target
type ToolchainModel struct {
	step           ToolchainStep
	textInput      textinput.Model
	spinner        spinner.Model
	cursor         int
	quitting       bool
	cancelled      bool
	errorMsg       string
	warnMsg        string
	checkingStatus string // Current phase: "pulling", "checking"

	// Existing targets (for validation)
	existingTargets map[string]bool

	// Configuration being built
	name         string
	runner       string
	dockerMode   string
	image        string
	dockerfile   string
	buildContext string
	platform     string
	buildType    string

	// Options
	runnerOptions     []string
	dockerModeOptions []string
	platformOptions   []string
	buildTypeOptions  []string

	// Docker image selection
	availableImages  []DockerImage
	filteredImages   []DockerImage
	imageFilter      string
	imageScrollStart int
	maxVisibleImages int

	// Answered questions
	questions       []Question
	currentQuestion string
}

// ToolchainConfig is the result of the TUI
type ToolchainConfig struct {
	Name         string
	Runner       string
	DockerMode   string
	Image        string
	Dockerfile   string
	BuildContext string
	Platform     string
	BuildType    string
}

// NewToolchainModel creates a new model for adding a CI target
func NewToolchainModel(existingTargets []string) ToolchainModel {
	ti := textinput.New()
	ti.Placeholder = "linux-amd64"
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 50
	ti.PromptStyle = inputPromptStyle
	ti.TextStyle = inputTextStyle
	ti.Cursor.Style = cursorStyle

	// Build existing targets map
	existing := make(map[string]bool)
	for _, t := range existingTargets {
		existing[t] = true
	}

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return ToolchainModel{
		step:              ToolchainStepName,
		textInput:         ti,
		spinner:           s,
		cursor:            0,
		existingTargets:   existing,
		currentQuestion:   "What should this target be called?",
		runnerOptions:     []string{"docker", "native"},
		dockerModeOptions: []string{"pull", "build", "local"},
		platformOptions:   []string{"linux/amd64", "linux/arm64", "linux/arm/v7", "None"},
		buildTypeOptions:  []string{"Release", "Debug", "RelWithDebInfo", "MinSizeRel"},
		runner:            "docker",
		dockerMode:        "pull",
		buildType:         "Release",
		buildContext:      ".",
		maxVisibleImages:  8,
	}
}

func (m ToolchainModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m ToolchainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't allow input during checking
		if m.step == ToolchainStepCheckingImage {
			if msg.String() == "ctrl+c" || msg.String() == "esc" {
				m.quitting = true
				m.cancelled = true
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.step == ToolchainStepDockerImageSelect {
				if m.cursor > 0 {
					m.cursor--
					// Scroll up if needed
					if m.cursor < m.imageScrollStart {
						m.imageScrollStart = m.cursor
					}
				}
			} else if !m.isTextInputStep() && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.step == ToolchainStepDockerImageSelect {
				maxCursor := len(m.filteredImages) - 1
				if maxCursor < 0 {
					maxCursor = 0
				}
				if m.cursor < maxCursor {
					m.cursor++
					// Scroll down if needed
					if m.cursor >= m.imageScrollStart+m.maxVisibleImages {
						m.imageScrollStart = m.cursor - m.maxVisibleImages + 1
					}
				}
			} else if !m.isTextInputStep() {
				maxCursor := m.getMaxCursor()
				if m.cursor < maxCursor {
					m.cursor++
				}
			}

		case "tab":
			// Tab to switch between filter input and selection in image select step
			if m.step == ToolchainStepDockerImageSelect {
				// Could toggle focus, but for now just keep filtering active
			}
		}

	case ImageCheckResult:
		// Handle async image check result
		if msg.Success {
			m.errorMsg = ""
			m.checkingStatus = ""
			// Proceed to build type selection
			m.currentQuestion = "Build type?"
			m.step = ToolchainStepBuildType
			m.cursor = 0
		} else {
			m.errorMsg = msg.Error
			m.checkingStatus = ""
			// Remove the last two questions (image and platform) to start fresh
			if len(m.questions) >= 2 {
				m.questions = m.questions[:len(m.questions)-2]
			}
			// Return to image name step to let user try a different image
			m.currentQuestion = "Docker image name/tag?"
			m.step = ToolchainStepDockerImage
			m.textInput.Reset()
			if m.dockerMode == "pull" {
				m.textInput.Placeholder = "ubuntu:22.04"
			} else {
				m.textInput.Placeholder = "my-local-image:latest"
			}
			m.textInput.Focus()
		}
		return m, nil

	case ImageCheckProgress:
		// Update the checking status display and trigger next phase
		m.checkingStatus = msg.Phase
		if msg.Phase == "checking" {
			// Image is ready, now check for build tools
			return m, tea.Batch(m.spinner.Tick, checkImageToolsCmd(m.image))
		}
		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Update text input if on text input steps
	if m.isTextInputStep() {
		m.textInput, cmd = m.textInput.Update(msg)

		// Update image filter when typing in image select step
		if m.step == ToolchainStepDockerImageSelect {
			newFilter := m.textInput.Value()
			if newFilter != m.imageFilter {
				m.imageFilter = newFilter
				m.filteredImages = filterImages(m.availableImages, m.imageFilter)
				m.cursor = 0
				m.imageScrollStart = 0
			}
		}
	}

	return m, cmd
}

func (m ToolchainModel) isTextInputStep() bool {
	return m.step == ToolchainStepName || m.step == ToolchainStepDockerImage ||
		m.step == ToolchainStepDockerfile || m.step == ToolchainStepBuildContext ||
		m.step == ToolchainStepDockerImageSelect
}

// checkDockerImageExists checks if a Docker image exists locally
func checkDockerImageExists(image string) bool {
	cmd := exec.Command("docker", "images", "-q", image)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// listDockerImages returns a list of available local Docker images
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
			// Skip <none> repositories
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

	// Fetch architectures for all images in one call
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

// getImageArchitectures fetches architecture info for multiple images
func getImageArchitectures(imageIDs []string) map[string]string {
	archMap := make(map[string]string)

	// Use docker inspect to get architecture for all images at once
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

// filterImages filters images based on a search string
func filterImages(images []DockerImage, filter string) []DockerImage {
	if filter == "" {
		return images
	}
	filter = strings.ToLower(filter)
	var filtered []DockerImage
	for _, img := range images {
		name := strings.ToLower(img.FullName())
		if strings.Contains(name, filter) {
			filtered = append(filtered, img)
		}
	}
	return filtered
}

// checkFileExists checks if a file exists
func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// checkCommandExists checks if a command is available in PATH
func checkCommandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// detectProjectType returns "vcpkg", "Bazel", "meson", or "CMake"
func detectProjectType() string {
	if checkFileExists("vcpkg.json") {
		return "vcpkg"
	}
	if checkFileExists("BUILD.bazel") || checkFileExists("WORKSPACE") || checkFileExists("MODULE.bazel") {
		return "bazel"
	}
	if checkFileExists("meson.build") {
		return "meson"
	}
	if checkFileExists("CMakeLists.txt") {
		return "cmake"
	}
	return "unknown"
}

// checkBuildToolsForProject checks if the required build tools are available for the project type
func checkBuildToolsForProject(projectType string) []string {
	var missing []string

	switch projectType {
	case "vcpkg", "cmake":
		if !checkCommandExists("cmake") {
			missing = append(missing, "cmake")
		}
		if !checkCommandExists("make") && !checkCommandExists("ninja") {
			missing = append(missing, "make or ninja")
		}
		hasCC := checkCommandExists("gcc") || checkCommandExists("clang") || checkCommandExists("cc")
		hasCXX := checkCommandExists("g++") || checkCommandExists("clang++") || checkCommandExists("c++")
		if !hasCC {
			missing = append(missing, "C compiler")
		}
		if !hasCXX {
			missing = append(missing, "C++ compiler")
		}
		if projectType == "vcpkg" {
			if os.Getenv("VCPKG_ROOT") == "" && !checkCommandExists("vcpkg") {
				missing = append(missing, "vcpkg")
			}
		}
	case "bazel":
		if !checkCommandExists("bazel") && !checkCommandExists("bazelisk") {
			missing = append(missing, "bazel or bazelisk")
		}
	case "meson":
		if !checkCommandExists("meson") {
			missing = append(missing, "meson")
		}
		if !checkCommandExists("ninja") {
			missing = append(missing, "ninja")
		}
		hasCC := checkCommandExists("gcc") || checkCommandExists("clang") || checkCommandExists("cc")
		hasCXX := checkCommandExists("g++") || checkCommandExists("clang++") || checkCommandExists("c++")
		if !hasCC {
			missing = append(missing, "C compiler")
		}
		if !hasCXX {
			missing = append(missing, "C++ compiler")
		}
	}

	return missing
}

// checkDockerImageHasCommand checks if a command exists inside a Docker image (with timeout)
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

// checkBuildToolsInDockerImage checks if build tools are available inside a Docker image
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

// ImageCheckPhase represents different phases of checking
type ImageCheckPhase int

const (
	CheckPhasePulling ImageCheckPhase = iota
	CheckPhaseChecking
)

// checkImagePullCmd attempts to pull/verify the image exists
func checkImagePullCmd(image, dockerMode, platform string) tea.Cmd {
	return func() tea.Msg {
		if dockerMode == "local" {
			// For local mode, just check if it exists
			if !checkDockerImageExists(image) {
				return ImageCheckResult{
					Success: false,
					Error:   fmt.Sprintf("Docker image not found locally: %s", image),
				}
			}
			// Image exists, now check tools
			return ImageCheckProgress{Phase: "checking"}
		}

		// For pull mode, try to pull the image with the specified platform
		var cmd *exec.Cmd
		if platform != "" {
			cmd = exec.Command("docker", "pull", "--platform", platform, image)
		} else {
			cmd = exec.Command("docker", "pull", image)
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Run()
		}()

		select {
		case err := <-done:
			if err != nil {
				return ImageCheckResult{
					Success: false,
					Error:   fmt.Sprintf("Failed to pull image: %s", image),
				}
			}
			// Pull succeeded, now check tools
			return ImageCheckProgress{Phase: "checking"}
		case <-time.After(120 * time.Second):
			_ = cmd.Process.Kill()
			return ImageCheckResult{
				Success: false,
				Error:   fmt.Sprintf("Timeout pulling image: %s", image),
			}
		}
	}
}

// checkImageToolsCmd checks if build tools are available in the image
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

// checkImageAsync runs the image validation asynchronously (initial pull phase)
func checkImageAsync(image, dockerMode, platform string) tea.Cmd {
	return checkImagePullCmd(image, dockerMode, platform)
}

func (m ToolchainModel) handleEnter() (tea.Model, tea.Cmd) {
	// Clear previous warnings
	m.warnMsg = ""

	switch m.step {
	case ToolchainStepName:
		name := strings.TrimSpace(m.textInput.Value())
		if name == "" {
			m.errorMsg = "Target name cannot be empty"
			return m, nil
		}
		if !isValidProjectName(name) {
			m.errorMsg = "Target name can only contain letters, numbers, hyphens, and underscores"
			return m, nil
		}
		if m.existingTargets[name] {
			m.errorMsg = fmt.Sprintf("Target '%s' already exists in cpx-ci.yaml", name)
			return m, nil
		}
		m.name = name
		m.errorMsg = ""

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   name,
			Complete: true,
		})

		m.currentQuestion = "Which runner should be used?"
		m.step = ToolchainStepRunner
		m.cursor = 0

	case ToolchainStepRunner:
		m.runner = m.runnerOptions[m.cursor]

		// For native runner, check if required build tools are available
		if m.runner == "native" {
			projectType := detectProjectType()
			missingTools := checkBuildToolsForProject(projectType)
			if len(missingTools) > 0 {
				m.errorMsg = fmt.Sprintf("Missing tools for %s project: %s", projectType, strings.Join(missingTools, ", "))
				return m, nil
			}
		}

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   m.runner,
			Complete: true,
		})

		if m.runner == "docker" {
			if !checkCommandExists("docker") {
				m.errorMsg = "Docker is not installed or not in PATH"
				return m, nil
			}
			m.currentQuestion = "Docker mode?"
			m.step = ToolchainStepDockerMode
			m.cursor = 0
		} else {
			m.currentQuestion = "Build type?"
			m.step = ToolchainStepBuildType
			m.cursor = 0
		}

	case ToolchainStepDockerMode:
		m.dockerMode = m.dockerModeOptions[m.cursor]

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   m.dockerMode,
			Complete: true,
		})

		if m.dockerMode == "build" {
			m.currentQuestion = "Dockerfile path?"
			m.step = ToolchainStepDockerfile

			m.textInput.Reset()
			m.textInput.Placeholder = "Dockerfile"
			m.textInput.Focus()
		} else if m.dockerMode == "local" {
			// For local mode, show interactive image selection
			m.availableImages = listDockerImages()
			m.filteredImages = m.availableImages
			m.imageFilter = ""
			m.cursor = 0
			m.imageScrollStart = 0

			if len(m.availableImages) == 0 {
				// No local images, fall back to text input
				m.currentQuestion = "Docker image name/tag?"
				m.step = ToolchainStepDockerImage
				m.textInput.Reset()
				m.textInput.Placeholder = "my-local-image:latest"
				m.textInput.Focus()
				m.warnMsg = "No local Docker images found"
			} else {
				m.currentQuestion = "Select Docker image (type to filter):"
				m.step = ToolchainStepDockerImageSelect
				m.textInput.Reset()
				m.textInput.Placeholder = "Type to filter..."
				m.textInput.Focus()
			}
		} else {
			// For pull mode, use text input
			m.currentQuestion = "Docker image name/tag?"
			m.step = ToolchainStepDockerImage

			m.textInput.Reset()
			m.textInput.Placeholder = "ubuntu:22.04"
			m.textInput.Focus()
		}

	case ToolchainStepDockerImageSelect:
		// User selected an image from the list
		if len(m.filteredImages) > 0 && m.cursor < len(m.filteredImages) {
			m.image = m.filteredImages[m.cursor].FullName()
		} else if m.imageFilter != "" {
			// Use the filter as a custom image name
			m.image = m.imageFilter
		} else {
			m.errorMsg = "Please select an image or enter a name"
			return m, nil
		}

		m.errorMsg = ""
		m.questions = append(m.questions, Question{
			Question: "Docker image?",
			Answer:   m.image,
			Complete: true,
		})

		// For local mode, skip platform selection and go to checking
		m.platform = "" // Local images use their native platform
		m.step = ToolchainStepCheckingImage
		m.checkingStatus = ""
		return m, tea.Batch(m.spinner.Tick, checkImageAsync(m.image, m.dockerMode, m.platform))

	case ToolchainStepDockerfile:
		dockerfile := strings.TrimSpace(m.textInput.Value())
		if dockerfile == "" {
			dockerfile = "Dockerfile"
		}

		if !checkFileExists(dockerfile) {
			m.errorMsg = fmt.Sprintf("Dockerfile not found: %s", dockerfile)
			return m, nil
		}

		m.dockerfile = dockerfile
		m.errorMsg = ""

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   dockerfile,
			Complete: true,
		})

		m.currentQuestion = "Build context directory?"
		m.step = ToolchainStepBuildContext

		m.textInput.Reset()
		m.textInput.Placeholder = "."
		m.textInput.Focus()

	case ToolchainStepBuildContext:
		context := strings.TrimSpace(m.textInput.Value())
		if context == "" {
			context = "."
		}

		info, err := os.Stat(context)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Directory not found: %s", context)
			return m, nil
		}
		if !info.IsDir() {
			m.errorMsg = fmt.Sprintf("Not a directory: %s", context)
			return m, nil
		}

		m.buildContext = context
		m.errorMsg = ""

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   context,
			Complete: true,
		})

		m.currentQuestion = "Tag for the built image?"
		m.step = ToolchainStepDockerImage

		m.textInput.Reset()
		m.textInput.Placeholder = "cpx-" + m.name
		m.textInput.Focus()

	case ToolchainStepDockerImage:
		image := strings.TrimSpace(m.textInput.Value())
		if image == "" {
			if m.dockerMode == "pull" {
				image = "ubuntu:22.04"
			} else {
				image = "cpx-" + m.name
			}
		}
		m.image = image

		m.errorMsg = ""
		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   image,
			Complete: true,
		})

		// For local mode, skip platform selection and go directly to checking
		if m.dockerMode == "local" {
			m.platform = ""
			m.step = ToolchainStepCheckingImage
			m.checkingStatus = ""
			return m, tea.Batch(m.spinner.Tick, checkImageAsync(m.image, m.dockerMode, m.platform))
		}

		// For pull/build modes, ask about platform
		m.currentQuestion = "Target platform?"
		m.step = ToolchainStepPlatform
		m.cursor = 0
		return m, nil

	case ToolchainStepPlatform:
		if m.cursor == len(m.platformOptions)-1 {
			m.platform = ""
		} else {
			m.platform = m.platformOptions[m.cursor]
		}

		answer := m.platformOptions[m.cursor]
		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		// For build mode, skip image check and go directly to build type
		if m.dockerMode == "build" {
			m.currentQuestion = "Build type?"
			m.step = ToolchainStepBuildType
			m.cursor = 0
			return m, nil
		}

		// For pull/local modes, now check the image with the correct platform
		m.step = ToolchainStepCheckingImage
		m.checkingStatus = ""
		return m, tea.Batch(m.spinner.Tick, checkImageAsync(m.image, m.dockerMode, m.platform))

	case ToolchainStepBuildType:
		m.buildType = m.buildTypeOptions[m.cursor]

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   m.buildType,
			Complete: true,
		})

		m.step = ToolchainStepDone
		return m, tea.Quit
	}

	return m, nil
}

func (m ToolchainModel) getMaxCursor() int {
	switch m.step {
	case ToolchainStepRunner:
		return len(m.runnerOptions) - 1
	case ToolchainStepDockerMode:
		return len(m.dockerModeOptions) - 1
	case ToolchainStepPlatform:
		return len(m.platformOptions) - 1
	case ToolchainStepBuildType:
		return len(m.buildTypeOptions) - 1
	default:
		return 0
	}
}

func (m ToolchainModel) View() string {
	if m.quitting && m.cancelled {
		return "\n  " + dimStyle.Render("Cancelled.") + "\n\n"
	}

	if m.step == ToolchainStepDone {
		return ""
	}

	var s strings.Builder

	// Header
	s.WriteString(dimStyle.Render("cpx add-toolchain") + "\n\n")

	// Title
	s.WriteString(cyanBold.Render("Add Toolchain") + "\n\n")

	// Render completed questions
	for _, q := range m.questions {
		s.WriteString(greenCheck.Render("✔") + " " + dimStyle.Render(q.Question) + " " + cyanBold.Render(q.Answer) + "\n")
	}

	// Show spinner during checking
	if m.step == ToolchainStepCheckingImage {
		if m.checkingStatus == "checking" {
			s.WriteString("\n" + m.spinner.View() + " " + questionStyle.Render("Checking build tools in image...") + "\n")
		} else {
			s.WriteString("\n" + m.spinner.View() + " " + questionStyle.Render("Pulling image...") + "\n")
			s.WriteString(dimStyle.Render("  (this may take a moment)") + "\n")
		}
	} else {
		// Render current question
		s.WriteString(questionMark.Render("?") + " " + questionStyle.Render(m.currentQuestion) + " ")

		switch m.step {
		case ToolchainStepName:
			s.WriteString(cyanBold.Render(m.textInput.View()))
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case ToolchainStepRunner:
			s.WriteString(dimStyle.Render(m.runnerOptions[m.cursor]))
			s.WriteString("\n")
			for i, opt := range m.runnerOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				desc := ""
				if opt == "docker" {
					desc = dimStyle.Render(" (build in container)")
				} else {
					desc = dimStyle.Render(" (build on host)")
				}
				s.WriteString(fmt.Sprintf("  %s %s%s\n", cursor, opt, desc))
			}
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case ToolchainStepDockerMode:
			s.WriteString(dimStyle.Render(m.dockerModeOptions[m.cursor]))
			s.WriteString("\n")
			for i, opt := range m.dockerModeOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				desc := ""
				switch opt {
				case "pull":
					desc = dimStyle.Render(" (pull image from registry)")
				case "build":
					desc = dimStyle.Render(" (build from Dockerfile)")
				case "local":
					desc = dimStyle.Render(" (use existing local image)")
				}
				s.WriteString(fmt.Sprintf("  %s %s%s\n", cursor, opt, desc))
			}

		case ToolchainStepDockerImageSelect:
			// Filter input
			s.WriteString("\n")
			s.WriteString("  " + dimStyle.Render("Filter: ") + cyanBold.Render(m.textInput.View()) + "\n")

			// Show available images count
			totalImages := len(m.availableImages)
			filteredCount := len(m.filteredImages)
			if m.imageFilter != "" {
				s.WriteString("  " + dimStyle.Render(fmt.Sprintf("Showing %d of %d images", filteredCount, totalImages)) + "\n")
			} else {
				s.WriteString("  " + dimStyle.Render(fmt.Sprintf("%d images available", totalImages)) + "\n")
			}
			s.WriteString("\n")

			// Show filtered images with scrolling
			if len(m.filteredImages) == 0 {
				if m.imageFilter != "" {
					s.WriteString("  " + dimStyle.Render("No images match filter. Press Enter to use '"+m.imageFilter+"' as image name.") + "\n")
				} else {
					s.WriteString("  " + dimStyle.Render("No images available") + "\n")
				}
			} else {
				// Calculate visible range
				start := m.imageScrollStart
				end := start + m.maxVisibleImages
				if end > len(m.filteredImages) {
					end = len(m.filteredImages)
				}

				// Show scroll indicator at top
				if start > 0 {
					s.WriteString("  " + dimStyle.Render("  ↑ more images above") + "\n")
				}

				for i := start; i < end; i++ {
					img := m.filteredImages[i]
					cursor := " "
					if m.cursor == i {
						cursor = selectedStyle.Render("❯")
					}

					// Format: repo:tag [arch]
					name := img.FullName()
					arch := img.Architecture
					if arch == "" {
						arch = "unknown"
					}
					archInfo := dimStyle.Render(fmt.Sprintf(" [%s]", arch))

					if m.cursor == i {
						s.WriteString(fmt.Sprintf("  %s %s%s\n", cursor, cyanBold.Render(name), archInfo))
					} else {
						s.WriteString(fmt.Sprintf("  %s %s%s\n", cursor, name, archInfo))
					}
				}

				// Show scroll indicator at bottom
				if end < len(m.filteredImages) {
					s.WriteString("  " + dimStyle.Render("  ↓ more images below") + "\n")
				}
			}

			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case ToolchainStepDockerfile:
			s.WriteString(cyanBold.Render(m.textInput.View()))
			s.WriteString("\n" + dimStyle.Render("  (e.g., Dockerfile, dockerfiles/Dockerfile.ubuntu)"))
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case ToolchainStepBuildContext:
			s.WriteString(cyanBold.Render(m.textInput.View()))
			s.WriteString("\n" + dimStyle.Render("  (e.g., . for current directory)"))
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case ToolchainStepDockerImage:
			s.WriteString(cyanBold.Render(m.textInput.View()))
			if m.dockerMode == "local" {
				s.WriteString("\n" + dimStyle.Render("  (must exist locally - use 'docker images' to check)"))
			}
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case ToolchainStepPlatform:
			s.WriteString(dimStyle.Render(m.platformOptions[m.cursor]))
			s.WriteString("\n")
			for i, opt := range m.platformOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				s.WriteString(fmt.Sprintf("  %s %s\n", cursor, opt))
			}

		case ToolchainStepBuildType:
			s.WriteString(dimStyle.Render(m.buildTypeOptions[m.cursor]))
			s.WriteString("\n")
			for i, opt := range m.buildTypeOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				s.WriteString(fmt.Sprintf("  %s %s\n", cursor, opt))
			}
		}
	}

	s.WriteString("\n\n" + dimStyle.Render("  Press Ctrl+C to cancel"))
	s.WriteString("\n")

	return s.String()
}

// GetConfig returns the target configuration
func (m ToolchainModel) GetConfig() ToolchainConfig {
	return ToolchainConfig{
		Name:         m.name,
		Runner:       m.runner,
		DockerMode:   m.dockerMode,
		Image:        m.image,
		Dockerfile:   m.dockerfile,
		BuildContext: m.buildContext,
		Platform:     m.platform,
		BuildType:    m.buildType,
	}
}

// IsCancelled returns true if the user canceled
func (m ToolchainModel) IsCancelled() bool {
	return m.cancelled
}

// ToCITarget converts the config to a CITarget
func (c ToolchainConfig) ToCITarget() config.Toolchain {
	target := config.Toolchain{
		Name:      c.Name,
		Runner:    c.Runner,
		BuildType: c.BuildType,
	}

	if c.Runner == "docker" {
		target.Docker = &config.DockerConfig{
			Mode:     c.DockerMode,
			Image:    c.Image,
			Platform: c.Platform,
		}

		if c.DockerMode == "build" {
			target.Docker.Build = &config.DockerBuildConfig{
				Context:    c.BuildContext,
				Dockerfile: c.Dockerfile,
			}
		}
	}

	return target
}

// RunAddTargetTUI runs the interactive TUI for adding a toolchain
func RunAddTargetTUI(existingTargets []string) (*ToolchainConfig, error) {
	m := NewToolchainModel(existingTargets)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	model := finalModel.(ToolchainModel)
	if model.IsCancelled() {
		return nil, nil
	}

	result := model.GetConfig()
	return &result, nil
}
