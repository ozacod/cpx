package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ToolchainStep represents the current step in the target creation flow
type ToolchainStep int

const (
	ToolchainStepName ToolchainStep = iota
	ToolchainStepRunner
	ToolchainStepBuildType
	ToolchainStepDone
)

// ToolchainModel represents the TUI state for adding a CI target
type ToolchainModel struct {
	step      ToolchainStep
	textInput textinput.Model
	spinner   spinner.Model
	cursor    int
	quitting  bool
	cancelled bool
	errorMsg  string

	// Existing targets (for validation)
	existingTargets map[string]bool

	// Configuration being built
	name      string
	runner    string
	buildType string

	// Options
	runnerOptions    []string
	buildTypeOptions []string

	// Answered questions
	questions       []Question
	currentQuestion string
}

// AddToolchainResult is the result of the TUI
type AddToolchainResult struct {
	Name      string
	Runner    string
	BuildType string
}

// NewToolchainModel creates a new model for adding a CI target
func NewToolchainModel(existingTargets []string, existingRunners []string) ToolchainModel {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 50
	ti.PromptStyle = inputPromptStyle
	ti.TextStyle = inputTextStyle
	ti.Cursor.Style = cursorStyle

	existing := make(map[string]bool)
	for _, t := range existingTargets {
		existing[t] = true
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	runnerOptions := []string{"Local System"}
	if len(existingRunners) > 0 {
		runnerOptions = append(runnerOptions, existingRunners...)
	}

	return ToolchainModel{
		step:             ToolchainStepName,
		textInput:        ti,
		spinner:          s,
		cursor:           0,
		existingTargets:  existing,
		currentQuestion:  "What should this target be called?",
		runnerOptions:    runnerOptions,
		buildTypeOptions: []string{"Release", "Debug", "RelWithDebInfo", "MinSizeRel"},
		runner:           "Local System",
		buildType:        "Release",
	}
}

func (m ToolchainModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ToolchainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ToolchainModel) isTextInputStep() bool {
	return m.step == ToolchainStepName
}

func (m ToolchainModel) handleEnter() (tea.Model, tea.Cmd) {
	m.errorMsg = ""

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

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   m.runner,
			Complete: true,
		})

		m.currentQuestion = "Build type?"
		m.step = ToolchainStepBuildType
		m.cursor = 0

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
			if opt == "Local System" {
				desc = dimStyle.Render(" (build on host)")
			} else {
				desc = dimStyle.Render(" (configured runner)")
			}
			s.WriteString(fmt.Sprintf("  %s %s%s\n", cursor, opt, desc))
		}
		if m.errorMsg != "" {
			s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
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

	s.WriteString("\n\n" + dimStyle.Render("Use arrow keys to navigate, Enter to select") + "\n")

	return s.String()
}

func (m ToolchainModel) GetResult() *AddToolchainResult {
	if m.cancelled {
		return nil
	}

	runner := m.runner
	if runner == "Local System" {
		runner = "" // Empty string implies native/local
	}

	return &AddToolchainResult{
		Name:      m.name,
		Runner:    runner,
		BuildType: m.buildType,
	}
}

func RunAddToolchainTUI(existingTargets []string, existingRunners []string) (*AddToolchainResult, error) {
	m := NewToolchainModel(existingTargets, existingRunners)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	return final.(ToolchainModel).GetResult(), nil
}

func (m ToolchainModel) IsCancelled() bool {
	return m.cancelled
}
