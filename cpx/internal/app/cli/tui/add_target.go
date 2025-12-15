package tui

import (
	"fmt"
	"strings"

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
	CIStepPlatform
	CIStepBuildType
	CIStepConfirm
	CIStepDone
)

// CITargetModel represents the TUI state for adding a CI target
type CITargetModel struct {
	step      CITargetStep
	textInput textinput.Model
	cursor    int
	quitting  bool
	cancelled bool
	errorMsg  string

	// Configuration being built
	name       string
	runner     string
	dockerMode string
	image      string
	platform   string
	buildType  string

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
	Name       string
	Runner     string
	DockerMode string
	Image      string
	Platform   string
	BuildType  string
}

// NewCITargetModel creates a new model for adding a CI target
func NewCITargetModel() CITargetModel {
	ti := textinput.New()
	ti.Placeholder = "linux-amd64"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40
	ti.PromptStyle = inputPromptStyle
	ti.TextStyle = inputTextStyle
	ti.Cursor.Style = cursorStyle

	return CITargetModel{
		step:              CIStepName,
		textInput:         ti,
		cursor:            0,
		currentQuestion:   "What should this target be called?",
		runnerOptions:     []string{"docker", "native"},
		dockerModeOptions: []string{"pull", "build", "local"},
		platformOptions:   []string{"linux/amd64", "linux/arm64", "linux/arm/v7", "None"},
		buildTypeOptions:  []string{"Release", "Debug", "RelWithDebInfo", "MinSizeRel"},
		runner:            "docker",
		dockerMode:        "pull",
		buildType:         "Release",
	}
}

func (m CITargetModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m CITargetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.step != CIStepName && m.step != CIStepDockerImage && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.step != CIStepName && m.step != CIStepDockerImage {
				maxCursor := m.getMaxCursor()
				if m.cursor < maxCursor {
					m.cursor++
				}
			}
		}
	}

	// Update text input if on text input steps
	if m.step == CIStepName || m.step == CIStepDockerImage {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m CITargetModel) handleEnter() (tea.Model, tea.Cmd) {
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

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   m.runner,
			Complete: true,
		})

		if m.runner == "docker" {
			m.currentQuestion = "Docker mode?"
			m.step = CIStepDockerMode
			m.cursor = 0
		} else {
			// Native runner, skip to build type
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

		m.currentQuestion = "Docker image name/tag?"
		m.step = CIStepDockerImage

		// Reset text input for image
		m.textInput.Reset()
		if m.dockerMode == "pull" {
			m.textInput.Placeholder = "ubuntu:22.04"
		} else {
			m.textInput.Placeholder = "cpx-" + m.name
		}
		m.textInput.Focus()

	case CIStepDockerImage:
		image := strings.TrimSpace(m.textInput.Value())
		if image == "" {
			// Use default
			if m.dockerMode == "pull" {
				image = "ubuntu:22.04"
			} else {
				image = "cpx-" + m.name
			}
		}
		m.image = image

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   image,
			Complete: true,
		})

		m.currentQuestion = "Target platform?"
		m.step = CIStepPlatform
		m.cursor = 0

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

		m.currentQuestion = "Build type?"
		m.step = CIStepBuildType
		m.cursor = 0

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
	s.WriteString(dimStyle.Render("cpx ci add-target") + "\n\n")

	// Title
	s.WriteString(cyanBold.Render("Add CI Target") + "\n\n")

	// Render completed questions
	for _, q := range m.questions {
		s.WriteString(greenCheck.Render("✔") + " " + dimStyle.Render(q.Question) + " " + cyanBold.Render(q.Answer) + "\n")
	}

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

	case CIStepDockerImage:
		s.WriteString(cyanBold.Render(m.textInput.View()))

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

	s.WriteString("\n\n" + dimStyle.Render("  Press Ctrl+C to cancel"))
	s.WriteString("\n")

	return s.String()
}

// GetConfig returns the target configuration
func (m CITargetModel) GetConfig() CITargetConfig {
	return CITargetConfig{
		Name:       m.name,
		Runner:     m.runner,
		DockerMode: m.dockerMode,
		Image:      m.image,
		Platform:   m.platform,
		BuildType:  m.buildType,
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
	}

	return target
}

// RunAddTarget runs the interactive TUI for adding a target
func RunAddTargetTUI() (*CITargetConfig, error) {
	m := NewCITargetModel()
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
