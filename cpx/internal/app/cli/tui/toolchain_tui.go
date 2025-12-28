package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ToolchainStep represents the current step in the target creation flow
type ToolchainStep int

const (
	ToolchainStepName ToolchainStep = iota
	ToolchainStepRunner
	ToolchainStepCompilerCC
	ToolchainStepCompilerCXX
	ToolchainStepCMakeToolchain
	ToolchainStepBuildType
	ToolchainStepDone
)

// ToolchainModel represents the TUI state for adding a CI target
type ToolchainModel struct {
	step      ToolchainStep
	textInput textinput.Model
	cursor    int
	cancelled bool

	// Data
	name           string
	runner         string
	buildType      string
	cc             string
	cxx            string
	cmakeToolchain string

	// Options
	existingNames    []string
	existingRunners  []string
	runnerOptions    []string
	buildTypeOptions []string
}

// AddToolchainResult is the result of the TUI
type AddToolchainResult struct {
	Name               string
	Runner             string
	BuildType          string
	CC                 string
	CXX                string
	CMakeToolchainFile string
}

// NewToolchainModel creates a new model for adding a CI target
func NewToolchainModel(existingNames []string, existingRunners []string) ToolchainModel {
	ti := textinput.New()
	ti.Placeholder = "my-toolchain"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40
	ti.TextStyle = inputTextStyle

	runnerOptions := []string{"Local System"}
	runnerOptions = append(runnerOptions, existingRunners...)

	return ToolchainModel{
		step:             ToolchainStepName,
		textInput:        ti,
		existingNames:    existingNames,
		existingRunners:  existingRunners,
		runnerOptions:    runnerOptions,
		buildTypeOptions: []string{"Release", "Debug", "RelWithDebInfo", "MinSizeRel"},
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
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "up", "k":
			if m.step == ToolchainStepRunner {
				if m.cursor > 0 {
					m.cursor--
				}
			} else if m.step == ToolchainStepBuildType {
				if m.cursor > 0 {
					m.cursor--
				}
			}
		case "down", "j":
			if m.step == ToolchainStepRunner {
				if m.cursor < len(m.runnerOptions)-1 {
					m.cursor++
				}
			} else if m.step == ToolchainStepBuildType {
				if m.cursor < len(m.buildTypeOptions)-1 {
					m.cursor++
				}
			}
		}
	}

	if m.step == ToolchainStepName || m.step == ToolchainStepCompilerCC || m.step == ToolchainStepCompilerCXX || m.step == ToolchainStepCMakeToolchain {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m ToolchainModel) handleEnter() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.textInput.Value())

	switch m.step {
	case ToolchainStepName:
		if val == "" {
			return m, nil // Required
		}
		// Check for duplicate name
		for _, n := range m.existingNames {
			if n == val {
				return m, nil // Duplicate
			}
		}
		m.name = val
		m.step = ToolchainStepRunner
		m.cursor = 0
		return m, nil

	case ToolchainStepRunner:
		m.runner = m.runnerOptions[m.cursor]
		if m.runner == "Local System" {
			m.runner = "" // Empty string means local/native
		}
		m.step = ToolchainStepCompilerCC
		m.textInput.Reset()
		m.textInput.Placeholder = "(optional, e.g. gcc-13)"
		m.textInput.Focus()
		return m, nil

	case ToolchainStepCompilerCC:
		m.cc = val
		m.step = ToolchainStepCompilerCXX
		m.textInput.Reset()
		m.textInput.Placeholder = "(optional, e.g. g++-13)"
		m.textInput.Focus()
		return m, nil

	case ToolchainStepCompilerCXX:
		m.cxx = val
		m.step = ToolchainStepCMakeToolchain
		m.textInput.Reset()
		m.textInput.Placeholder = "(optional path to toolchain file)"
		m.textInput.Focus()
		return m, nil

	case ToolchainStepCMakeToolchain:
		m.cmakeToolchain = val
		m.step = ToolchainStepBuildType
		m.cursor = 0
		return m, nil

	case ToolchainStepBuildType:
		m.buildType = m.buildTypeOptions[m.cursor]
		m.step = ToolchainStepDone
		return m, tea.Quit
	}

	return m, nil
}

func (m ToolchainModel) View() string {
	if m.step == ToolchainStepDone {
		return ""
	}

	var s strings.Builder

	// Header - removed as per user request
	// s.WriteString(titleStyle.Render("Add New Toolchain") + "\n\n")

	switch m.step {
	case ToolchainStepName:
		s.WriteString(questionStyle.Render("? Toolchain Name") + "\n")
		s.WriteString(m.textInput.View() + "\n")
		s.WriteString(helpStyle.Render("Enter a unique name for this toolchain"))

	case ToolchainStepRunner:
		s.WriteString(questionStyle.Render("? Select Runner") + "\n")
		for i, opt := range m.runnerOptions {
			cursor := " "
			style := textStyle
			if m.cursor == i {
				cursor = ">"
				style = selectedStyle
			}
			s.WriteString(fmt.Sprintf("%s %s\n", cursor, style.Render(opt)))
		}

	case ToolchainStepCompilerCC:
		s.WriteString(questionStyle.Render("? C Compiler (CC)") + "\n")
		s.WriteString(m.textInput.View() + "\n")
		s.WriteString(helpStyle.Render("Leave empty to use default"))

	case ToolchainStepCompilerCXX:
		s.WriteString(questionStyle.Render("? C++ Compiler (CXX)") + "\n")
		s.WriteString(m.textInput.View() + "\n")
		s.WriteString(helpStyle.Render("Leave empty to use default"))

	case ToolchainStepCMakeToolchain:
		s.WriteString(questionStyle.Render("? CMake Toolchain File") + "\n")
		s.WriteString(m.textInput.View() + "\n")
		s.WriteString(helpStyle.Render("Path to a CMake toolchain file (optional)"))

	case ToolchainStepBuildType:
		s.WriteString(questionStyle.Render("? Build Type") + "\n")
		for i, opt := range m.buildTypeOptions {
			cursor := " "
			style := textStyle
			if m.cursor == i {
				cursor = ">"
				style = selectedStyle
			}
			s.WriteString(fmt.Sprintf("%s %s\n", cursor, style.Render(opt)))
		}
	}

	// Minimized footer whitespace
	s.WriteString("\n" + helpStyle.Render("Press Esc to cancel, Enter to confirm"))

	return s.String()
}

func (m ToolchainModel) GetResult() *AddToolchainResult {
	if m.cancelled {
		return nil
	}
	return &AddToolchainResult{
		Name:               m.name,
		Runner:             m.runner,
		BuildType:          m.buildType,
		CC:                 m.cc,
		CXX:                m.cxx,
		CMakeToolchainFile: m.cmakeToolchain,
	}
}

func RunAddToolchainTUI(existingNames []string, existingRunners []string) (*AddToolchainResult, error) {
	m := NewToolchainModel(existingNames, existingRunners)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	res := final.(ToolchainModel).GetResult()
	return res, nil
}

func (m ToolchainModel) IsCancelled() bool {
	return m.cancelled
}
