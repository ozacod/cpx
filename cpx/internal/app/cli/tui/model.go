package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Step represents the current step in the project creation flow
type Step int

const (
	StepProjectName Step = iota
	StepProjectType
	StepCppStandard
	StepTestFramework
	StepBenchmark
	StepClangFormat
	StepPackageManager
	StepGitHooks
	StepPreCommit
	StepPrePush
	StepCreating
	StepDone
)

// Question represents a single question with its answer
type Question struct {
	Question string
	Answer   string
	Complete bool
}

// ProjectConfig holds the user's choices
type ProjectConfig struct {
	Name           string
	IsLibrary      bool
	CppStandard    int
	TestFramework  string
	Benchmark      string
	ClangFormat    string
	PackageManager string // "vcpkg" or "none"
	VCS            string // "git" or "none"
	UseHooks       bool
	GitHooks       []string
	PreCommit      []string
	PrePush        []string
}

// CreationMsg indicates project creation started
type CreationMsg struct{}

// CreationResultMsg contains creation result
type CreationResultMsg struct {
	Success bool
	Message string
}

// Model represents the TUI state
type Model struct {
	step      Step
	config    ProjectConfig
	textInput textinput.Model
	spinner   spinner.Model
	cursor    int
	err       error
	quitting  bool
	cancelled bool
	created   bool
	errorMsg  string

	// Answered questions history
	questions []Question

	// Current question state
	currentQuestion string

	// Options for selection steps
	projectTypeOptions    []string
	cppStandardOptions    []int
	testFrameworkOptions  []string
	benchmarkOptions      []string
	clangFormatOptions    []string
	packageManagerOptions []string
	preCommitOptions      []string
	prePushOptions        []string
	selectedPreCommit     map[int]bool
	selectedPrePush       map[int]bool

	// Creation result
	creationResult string
}

// InitialModel creates the initial model
func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "my-awesome-project"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40
	ti.PromptStyle = inputPromptStyle
	ti.TextStyle = inputTextStyle
	ti.Cursor.Style = cursorStyle

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return Model{
		step:                  StepProjectName,
		textInput:             ti,
		spinner:               s,
		cursor:                0,
		questions:             []Question{},
		currentQuestion:       "What will your project be called?",
		projectTypeOptions:    []string{"Executable", "Library"},
		cppStandardOptions:    []int{11, 14, 17, 20, 23},
		testFrameworkOptions:  []string{"GoogleTest", "Catch2", "doctest", "None"},
		benchmarkOptions:      []string{"Google Benchmark", "nanobench", "Catch2 benchmark", "None"},
		clangFormatOptions:    []string{"Google", "LLVM", "Chromium", "Mozilla", "WebKit"},
		packageManagerOptions: []string{"vcpkg", "Bazel", "None"},
		preCommitOptions:      []string{"format", "lint", "cppcheck", "test"},
		prePushOptions:        []string{"test", "cppcheck"},
		selectedPreCommit:     map[int]bool{0: true, 1: true},
		selectedPrePush:       map[int]bool{0: true},
		config: ProjectConfig{
			CppStandard:    17,
			TestFramework:  "googletest",
			Benchmark:      "none",
			ClangFormat:    "Google",
			PackageManager: "vcpkg",
			IsLibrary:      false,
			VCS:            "git",
			UseHooks:       false,
			GitHooks:       []string{},
			PreCommit:      []string{},
			PrePush:        []string{},
		},
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// tickCreation simulates project creation
func tickCreation() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return CreationResultMsg{Success: true, Message: ""}
	})
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.step != StepProjectName && m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.step != StepProjectName {
				maxCursor := m.getMaxCursor()
				if m.cursor < maxCursor {
					m.cursor++
				}
			}

		case " ":
			// Space to toggle for multi-select steps
			if m.step == StepPreCommit {
				m.selectedPreCommit[m.cursor] = !m.selectedPreCommit[m.cursor]
			} else if m.step == StepPrePush {
				m.selectedPrePush[m.cursor] = !m.selectedPrePush[m.cursor]
			}
		}

	case CreationResultMsg:
		m.step = StepDone
		m.created = msg.Success
		m.creationResult = msg.Message
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case error:
		m.err = msg
		return m, nil
	}

	// Update text input if on project name step
	if m.step == StepProjectName {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

// handleEnter processes the enter key based on current step
func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepProjectName:
		name := strings.TrimSpace(m.textInput.Value())
		if name == "" {
			m.errorMsg = "Project name cannot be empty"
			return m, nil
		}
		if !isValidProjectName(name) {
			m.errorMsg = "Project name can only contain letters, numbers, hyphens, and underscores"
			return m, nil
		}
		m.config.Name = name
		m.errorMsg = ""

		// Add answered question
		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   name,
			Complete: true,
		})

		m.currentQuestion = "What type of project would you like to create?"
		m.step = StepProjectType
		m.cursor = 0

	case StepProjectType:
		m.config.IsLibrary = m.cursor == 1
		answer := m.projectTypeOptions[m.cursor]

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		m.currentQuestion = "Which C++ standard would you like to use?"
		m.step = StepCppStandard
		m.cursor = 2 // Default to C++17

	case StepCppStandard:
		m.config.CppStandard = m.cppStandardOptions[m.cursor]
		answer := fmt.Sprintf("C++%d", m.config.CppStandard)

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		m.currentQuestion = "Which testing framework would you like to use?"
		m.step = StepTestFramework
		m.cursor = 0

	case StepTestFramework:
		frameworks := []string{"googletest", "catch2", "doctest", "none"}
		m.config.TestFramework = frameworks[m.cursor]
		answer := m.testFrameworkOptions[m.cursor]

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		m.currentQuestion = "Which benchmark framework would you like?"
		m.step = StepBenchmark
		m.cursor = 0

	case StepBenchmark:
		benchmarks := []string{"google-benchmark", "nanobench", "catch2-benchmark", "none"}
		m.config.Benchmark = benchmarks[m.cursor]
		answer := m.benchmarkOptions[m.cursor]

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		m.currentQuestion = "Which clang-format style would you like?"
		m.step = StepClangFormat
		m.cursor = 0

	case StepClangFormat:
		m.config.ClangFormat = m.clangFormatOptions[m.cursor]
		answer := m.config.ClangFormat

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		m.currentQuestion = "Would you like to use a package manager?"
		m.step = StepPackageManager
		m.cursor = 0

	case StepPackageManager:
		switch m.cursor {
		case 0:
			m.config.PackageManager = "vcpkg"
		case 1:
			m.config.PackageManager = "bazel"
		default:
			m.config.PackageManager = "none"
		}
		answer := m.packageManagerOptions[m.cursor]

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		m.currentQuestion = "Initialize a new git repository?"
		m.step = StepGitHooks
		m.cursor = 0

	case StepGitHooks:
		if m.cursor == 0 {
			m.config.VCS = "git"
			m.config.UseHooks = true
		} else {
			m.config.VCS = "none"
			m.config.UseHooks = false
		}
		answer := "Yes"
		if m.config.VCS == "none" {
			answer = "No"
		}

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		if m.config.VCS == "git" && m.config.UseHooks {
			m.currentQuestion = "Select pre-commit hooks:"
			m.step = StepPreCommit
			m.cursor = 0
		} else {
			// Start creating
			if m.config.VCS == "none" {
				m.questions = append(m.questions, Question{
					Question: "Sounds good! You can come back and run git init later.",
					Answer:   "",
					Complete: true,
				})
			}
			m.step = StepCreating
			return m, tickCreation()
		}

	case StepPreCommit:
		hookMap := []string{"fmt", "lint", "cppcheck", "test"}
		m.config.PreCommit = []string{}
		var selected []string
		for i, sel := range m.selectedPreCommit {
			if sel && i < len(hookMap) {
				m.config.PreCommit = append(m.config.PreCommit, hookMap[i])
				selected = append(selected, m.preCommitOptions[i])
			}
		}

		answer := strings.Join(selected, ", ")
		if answer == "" {
			answer = "None"
		}

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		m.currentQuestion = "Select pre-push hooks:"
		m.step = StepPrePush
		m.cursor = 0

	case StepPrePush:
		hookMap := []string{"test", "cppcheck"}
		m.config.PrePush = []string{}
		var selected []string
		for i, sel := range m.selectedPrePush {
			if sel && i < len(hookMap) {
				m.config.PrePush = append(m.config.PrePush, hookMap[i])
				selected = append(selected, m.prePushOptions[i])
			}
		}

		answer := strings.Join(selected, ", ")
		if answer == "" {
			answer = "None"
		}

		m.questions = append(m.questions, Question{
			Question: m.currentQuestion,
			Answer:   answer,
			Complete: true,
		})

		m.questions = append(m.questions, Question{
			Question: "Sounds good! You can come back and run git init later.",
			Answer:   "",
			Complete: true,
		})

		// Start creating
		m.step = StepCreating
		return m, tickCreation()
	}

	return m, nil
} // getMaxCursor returns the maximum cursor position for current step
func (m Model) getMaxCursor() int {
	switch m.step {
	case StepProjectType:
		return len(m.projectTypeOptions) - 1
	case StepCppStandard:
		return len(m.cppStandardOptions) - 1
	case StepTestFramework:
		return len(m.testFrameworkOptions) - 1
	case StepClangFormat:
		return len(m.clangFormatOptions) - 1
	case StepBenchmark:
		return len(m.benchmarkOptions) - 1
	case StepPackageManager:
		return len(m.packageManagerOptions) - 1
	case StepGitHooks:
		return 1 // Yes or No
	case StepPreCommit:
		return len(m.preCommitOptions) - 1
	case StepPrePush:
		return len(m.prePushOptions) - 1
	default:
		return 0
	}
}

// View renders the UI
func (m Model) View() string {
	if m.quitting && !m.created && m.cancelled {
		return "\n  " + dimStyle.Render("Cancelled.") + "\n\n"
	}

	if m.step == StepDone {
		return ""
	}

	var s strings.Builder

	// Command at top
	s.WriteString(dimStyle.Render("cpx new") + "\n\n")

	// ASCII Art Logo (smaller)
	logo := cyanBold.Render(` ██████ ██████  ██   ██ 
██      ██   ██  ██ ██  
██      ██████    ███   
██      ██       ██ ██  
 ██████ ██      ██   ██`)

	s.WriteString(logo + "\n\n")

	// Render all completed questions
	for _, q := range m.questions {
		if q.Answer != "" {
			s.WriteString(greenCheck.Render("✔") + " " + dimStyle.Render(q.Question) + " " + cyanBold.Render(q.Answer) + "\n")
		} else {
			// Message without answer
			s.WriteString(greenStyle.Render(q.Question) + "\n")
		}
	}

	// Render current question
	if m.step == StepCreating {
		s.WriteString("\n" + m.spinner.View() + " " + questionStyle.Render("Scaffolding your project...") + "\n")
	} else if m.step == StepDone {
		if m.created {
			s.WriteString("\n" + greenCheck.Render("✓") + " " + greenStyle.Render("Your project is ready!") + "\n")
		}
	} else {
		s.WriteString(questionMark.Render("?") + " " + questionStyle.Render(m.currentQuestion) + " ")

		// Show current answer being typed or selected
		switch m.step {
		case StepProjectName:
			s.WriteString(cyanBold.Render(m.textInput.View()))
			if m.errorMsg != "" {
				s.WriteString("\n  " + errorStyle.Render("✗ "+m.errorMsg))
			}

		case StepProjectType:
			s.WriteString(dimStyle.Render(m.projectTypeOptions[m.cursor]))
			s.WriteString("\n")
			for i, opt := range m.projectTypeOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				s.WriteString(fmt.Sprintf("  %s %s\n", cursor, opt))
			}

		case StepCppStandard:
			s.WriteString(dimStyle.Render(fmt.Sprintf("C++%d", m.cppStandardOptions[m.cursor])))
			s.WriteString("\n")
			for i, std := range m.cppStandardOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				s.WriteString(fmt.Sprintf("  %s C++%d\n", cursor, std))
			}

		case StepTestFramework:
			s.WriteString(dimStyle.Render(m.testFrameworkOptions[m.cursor]))
			s.WriteString("\n")
			for i, fw := range m.testFrameworkOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				s.WriteString(fmt.Sprintf("  %s %s\n", cursor, fw))
			}

		case StepBenchmark:
			s.WriteString(dimStyle.Render(m.benchmarkOptions[m.cursor]))
			s.WriteString("\n")
			for i, b := range m.benchmarkOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				s.WriteString(fmt.Sprintf("  %s %s\n", cursor, b))
			}

		case StepClangFormat:
			s.WriteString(dimStyle.Render(m.clangFormatOptions[m.cursor]))
			s.WriteString("\n")
			for i, style := range m.clangFormatOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				s.WriteString(fmt.Sprintf("  %s %s\n", cursor, style))
			}

		case StepPackageManager:
			s.WriteString(dimStyle.Render(m.packageManagerOptions[m.cursor]))
			s.WriteString("\n")
			for i, opt := range m.packageManagerOptions {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				s.WriteString(fmt.Sprintf("  %s %s\n", cursor, opt))
			}

		case StepGitHooks:
			answer := "Yes"
			if m.cursor == 1 {
				answer = "No"
			}
			s.WriteString(dimStyle.Render(answer))
			s.WriteString("\n")
			s.WriteString(fmt.Sprintf("  %s Yes\n", m.renderCursor(0)))
			s.WriteString(fmt.Sprintf("  %s No\n", m.renderCursor(1)))

		case StepPreCommit, StepPrePush:
			s.WriteString("\n")
			options := m.preCommitOptions
			selected := m.selectedPreCommit
			if m.step == StepPrePush {
				options = m.prePushOptions
				selected = m.selectedPrePush
			}

			for i, opt := range options {
				cursor := " "
				if m.cursor == i {
					cursor = selectedStyle.Render("❯")
				}
				checkbox := "◯"
				if selected[i] {
					checkbox = greenCheck.Render("◉")
				}
				s.WriteString(fmt.Sprintf("  %s %s %s\n", cursor, checkbox, opt))
			}
			s.WriteString("\n" + dimStyle.Render("  Space to select, Enter to continue"))
		}
	}

	s.WriteString("\n\n" + dimStyle.Render("  Press Ctrl+C to cancel"))
	s.WriteString("\n")

	return s.String()
}

// GetConfig returns the final configuration
func (m Model) GetConfig() ProjectConfig {
	return m.config
}

// IsCancelled returns true if the user cancelled
func (m Model) IsCancelled() bool {
	return m.cancelled
}

// Helper function to validate project name
func isValidProjectName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
