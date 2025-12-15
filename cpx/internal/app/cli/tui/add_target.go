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

// CITargetStep represents the current step in the target creation flow
type CITargetStep int

const (
	CIStepName CITargetStep = iota
	CIStepRunner
	CIStepDockerMode
	CIStepDockerImage
	CIStepCheckingImage // New: async checking step
	CIStepDockerfile    // for build mode
	CIStepBuildContext  // for build mode
	CIStepPlatform
	CIStepBuildType
	CIStepConfirm
	CIStepDone
)

// ImageCheckResult is the result of async image checking
type ImageCheckResult struct {
	Success bool
	Error   string
}

// ImageCheckProgress is sent during async checking to update status
type ImageCheckProgress struct {
	Phase string // "pulling", "checking"
}

// CITargetModel represents the TUI state for adding a CI target
type CITargetModel struct {
	step           CITargetStep
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

	// Answered questions
	questions       []Question
	currentQuestion string
}

// CITargetConfig is the result of the TUI
type CITargetConfig struct {
	Name         string
	Runner       string
	DockerMode   string
	Image        string
	Dockerfile   string
	BuildContext string
	Platform     string
	BuildType    string
}

// NewCITargetModel creates a new model for adding a CI target
func NewCITargetModel(existingTargets []string) CITargetModel {
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

	return CITargetModel{
		step:              CIStepName,
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
	}
}

func (m CITargetModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m CITargetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't allow input during checking
		if m.step == CIStepCheckingImage {
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
			if !m.isTextInputStep() && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if !m.isTextInputStep() {
				maxCursor := m.getMaxCursor()
				if m.cursor < maxCursor {
					m.cursor++
				}
			}
		}

	case ImageCheckResult:
		// Handle async image check result
		if msg.Success {
			m.errorMsg = ""
			m.checkingStatus = ""
			// Proceed to build type selection
			m.currentQuestion = "Build type?"
			m.step = CIStepBuildType
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
			m.step = CIStepDockerImage
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
	}

	return m, cmd
}

func (m CITargetModel) isTextInputStep() bool {
	return m.step == CIStepName || m.step == CIStepDockerImage ||
		m.step == CIStepDockerfile || m.step == CIStepBuildContext
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

// detectProjectType returns "vcpkg", "bazel", "meson", or "cmake"
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
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
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
			cmd.Process.Kill()
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

func (m CITargetModel) handleEnter() (tea.Model, tea.Cmd) {
	// Clear previous warnings
	m.warnMsg = ""

	switch m.step {
	case CIStepName:
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
		m.step = CIStepRunner
		m.cursor = 0

	case CIStepRunner:
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
			m.step = CIStepDockerMode
			m.cursor = 0
		} else {
			m.currentQuestion = "Build type?"
			m.step = CIStepBuildType
			m.cursor = 0
		}

	case CIStepDockerMode:
		m.dockerMode = m.dockerModeOptions[m.cursor]

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   m.dockerMode,
			Complete: true,
		})

		if m.dockerMode == "build" {
			m.currentQuestion = "Dockerfile path?"
			m.step = CIStepDockerfile

			m.textInput.Reset()
			m.textInput.Placeholder = "Dockerfile"
			m.textInput.Focus()
		} else {
			m.currentQuestion = "Docker image name/tag?"
			m.step = CIStepDockerImage

			m.textInput.Reset()
			if m.dockerMode == "pull" {
				m.textInput.Placeholder = "ubuntu:22.04"
			} else {
				m.textInput.Placeholder = "my-local-image:latest"
			}
			m.textInput.Focus()
		}

	case CIStepDockerfile:
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
		m.step = CIStepBuildContext

		m.textInput.Reset()
		m.textInput.Placeholder = "."
		m.textInput.Focus()

	case CIStepBuildContext:
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
		m.step = CIStepDockerImage

		m.textInput.Reset()
		m.textInput.Placeholder = "cpx-" + m.name
		m.textInput.Focus()

	case CIStepDockerImage:
		image := strings.TrimSpace(m.textInput.Value())
		if image == "" {
			if m.dockerMode == "pull" {
				image = "ubuntu:22.04"
			} else {
				image = "cpx-" + m.name
			}
		}
		m.image = image

		// For all modes, proceed to platform selection first
		m.errorMsg = ""
		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   image,
			Complete: true,
		})
		m.currentQuestion = "Target platform?"
		m.step = CIStepPlatform
		m.cursor = 0
		return m, nil

	case CIStepPlatform:
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
			m.step = CIStepBuildType
			m.cursor = 0
			return m, nil
		}

		// For pull/local modes, now check the image with the correct platform
		m.step = CIStepCheckingImage
		m.checkingStatus = ""
		return m, tea.Batch(m.spinner.Tick, checkImageAsync(m.image, m.dockerMode, m.platform))

	case CIStepBuildType:
		m.buildType = m.buildTypeOptions[m.cursor]

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   m.buildType,
			Complete: true,
		})

		m.step = CIStepDone
		return m, tea.Quit
	}

	return m, nil
}

func (m CITargetModel) getMaxCursor() int {
	switch m.step {
	case CIStepRunner:
		return len(m.runnerOptions) - 1
	case CIStepDockerMode:
		return len(m.dockerModeOptions) - 1
	case CIStepPlatform:
		return len(m.platformOptions) - 1
	case CIStepBuildType:
		return len(m.buildTypeOptions) - 1
	default:
		return 0
	}
}

func (m CITargetModel) View() string {
	if m.quitting && m.cancelled {
		return "\n  " + dimStyle.Render("Cancelled.") + "\n\n"
	}

	if m.step == CIStepDone {
		return ""
	}

	var s strings.Builder

	// Header
	s.WriteString(dimStyle.Render("cpx add-target") + "\n\n")

	// Title
	s.WriteString(cyanBold.Render("Add CI Target") + "\n\n")

	// Render completed questions
	for _, q := range m.questions {
		s.WriteString(greenCheck.Render("✔") + " " + dimStyle.Render(q.Question) + " " + cyanBold.Render(q.Answer) + "\n")
	}

	// Show spinner during checking
	if m.step == CIStepCheckingImage {
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
		case CIStepName:
			s.WriteString(cyanBold.Render(m.textInput.View()))
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case CIStepRunner:
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

		case CIStepDockerMode:
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

		case CIStepDockerfile:
			s.WriteString(cyanBold.Render(m.textInput.View()))
			s.WriteString("\n" + dimStyle.Render("  (e.g., Dockerfile, dockerfiles/Dockerfile.ubuntu)"))
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case CIStepBuildContext:
			s.WriteString(cyanBold.Render(m.textInput.View()))
			s.WriteString("\n" + dimStyle.Render("  (e.g., . for current directory)"))
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case CIStepDockerImage:
			s.WriteString(cyanBold.Render(m.textInput.View()))
			if m.dockerMode == "local" {
				s.WriteString("\n" + dimStyle.Render("  (must exist locally - use 'docker images' to check)"))
			}
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case CIStepPlatform:
			s.WriteString(dimStyle.Render(m.platformOptions[m.cursor]))
			s.WriteString("\n")
			for i, opt := range m.platformOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				s.WriteString(fmt.Sprintf("  %s %s\n", cursor, opt))
			}

		case CIStepBuildType:
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
func (m CITargetModel) GetConfig() CITargetConfig {
	return CITargetConfig{
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

// IsCancelled returns true if the user cancelled
func (m CITargetModel) IsCancelled() bool {
	return m.cancelled
}

// ToCITarget converts the config to a CITarget
func (c CITargetConfig) ToCITarget() config.CITarget {
	target := config.CITarget{
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

// RunAddTargetTUI runs the interactive TUI for adding a target
func RunAddTargetTUI(existingTargets []string) (*CITargetConfig, error) {
	m := NewCITargetModel(existingTargets)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	model := finalModel.(CITargetModel)
	if model.IsCancelled() {
		return nil, nil
	}

	result := model.GetConfig()
	return &result, nil
}
