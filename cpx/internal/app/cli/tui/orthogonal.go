package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// =========================================
// Add Toolchain TUI (creates build configuration)
// =========================================

type AddToolchainStep int

const (
	addToolchainStepName AddToolchainStep = iota
	addToolchainStepRunner
	addToolchainStepBuildType
	addToolchainStepDone
)

type AddToolchainModel struct {
	step          AddToolchainStep
	textInput     textinput.Model
	cursor        int
	quitting      bool
	cancelled     bool
	errorMsg      string
	existingNames map[string]bool
	runnerNames   []string
	buildTypes    []string
	name          string
	runner        string
	buildType     string
}

type AddToolchainResult struct {
	Name      string
	Runner    string
	BuildType string
}

func NewAddToolchainModel(existingNames []string, runnerNames []string) AddToolchainModel {
	ti := textinput.New()
	ti.Placeholder = "linux-release"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40
	ti.TextStyle = inputTextStyle

	existing := make(map[string]bool)
	for _, n := range existingNames {
		existing[n] = true
	}

	// Add "(local)" option to runner names
	runners := []string{"(local)"}
	runners = append(runners, runnerNames...)

	return AddToolchainModel{
		step:          addToolchainStepName,
		textInput:     ti,
		existingNames: existing,
		runnerNames:   runners,
		buildTypes:    []string{"Release", "Debug", "RelWithDebInfo", "MinSizeRel"},
	}
}

func (m AddToolchainModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m AddToolchainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.step == addToolchainStepRunner || m.step == addToolchainStepBuildType {
				m.cursor--
				if m.cursor < 0 {
					if m.step == addToolchainStepRunner {
						m.cursor = len(m.runnerNames) - 1
					} else {
						m.cursor = len(m.buildTypes) - 1
					}
				}
				return m, nil
			}
		case "down", "j":
			if m.step == addToolchainStepRunner || m.step == addToolchainStepBuildType {
				m.cursor++
				if m.step == addToolchainStepRunner && m.cursor >= len(m.runnerNames) {
					m.cursor = 0
				} else if m.step == addToolchainStepBuildType && m.cursor >= len(m.buildTypes) {
					m.cursor = 0
				}
				return m, nil
			}
		}
	}

	if m.step == addToolchainStepName {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m AddToolchainModel) handleEnter() (tea.Model, tea.Cmd) {
	m.errorMsg = ""
	value := strings.TrimSpace(m.textInput.Value())

	switch m.step {
	case addToolchainStepName:
		if value == "" {
			m.errorMsg = "Name is required"
			return m, nil
		}
		if m.existingNames[value] {
			m.errorMsg = fmt.Sprintf("Toolchain '%s' already exists", value)
			return m, nil
		}
		m.name = value
		m.step = addToolchainStepRunner
		m.cursor = 0

	case addToolchainStepRunner:
		selected := m.runnerNames[m.cursor]
		if selected == "(local)" {
			m.runner = ""
		} else {
			m.runner = selected
		}
		m.step = addToolchainStepBuildType
		m.cursor = 0

	case addToolchainStepBuildType:
		m.buildType = m.buildTypes[m.cursor]
		m.step = addToolchainStepDone
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m AddToolchainModel) View() string {
	if m.quitting && m.cancelled {
		return "\n  " + dimStyle.Render("Cancelled.") + "\n\n"
	}
	if m.step == addToolchainStepDone {
		return ""
	}

	var s strings.Builder
	s.WriteString("\n")

	// Show answered questions
	if m.name != "" {
		s.WriteString("  " + successStyle.Render("✓") + " Toolchain name: " + m.name + "\n")
	}
	if m.step > addToolchainStepRunner {
		r := m.runner
		if r == "" {
			r = "(local)"
		}
		s.WriteString("  " + successStyle.Render("✓") + " Runner: " + r + "\n")
	}

	switch m.step {
	case addToolchainStepName:
		s.WriteString("\n  " + questionStyle.Render("? Toolchain name") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")

	case addToolchainStepRunner:
		s.WriteString("\n  " + questionStyle.Render("? Runner") + " " + dimStyle.Render("(execution environment)") + "\n")
		for i, opt := range m.runnerNames {
			cursor := "  "
			if m.cursor == i {
				cursor = selectedStyle.Render("❯ ")
				s.WriteString("  " + cursor + selectedStyle.Render(opt) + "\n")
			} else {
				s.WriteString("  " + cursor + dimStyle.Render(opt) + "\n")
			}
		}

	case addToolchainStepBuildType:
		s.WriteString("\n  " + questionStyle.Render("? Build type") + "\n")
		for i, opt := range m.buildTypes {
			cursor := "  "
			if m.cursor == i {
				cursor = selectedStyle.Render("❯ ")
				s.WriteString("  " + cursor + selectedStyle.Render(opt) + "\n")
			} else {
				s.WriteString("  " + cursor + dimStyle.Render(opt) + "\n")
			}
		}
	}

	if m.errorMsg != "" {
		s.WriteString("  " + errorStyle.Render("✗ "+m.errorMsg) + "\n")
	}

	s.WriteString("\n  " + dimStyle.Render("Enter to confirm • ↑↓ to select • Esc to cancel") + "\n")
	return s.String()
}

func (m AddToolchainModel) GetResult() *AddToolchainResult {
	if m.cancelled {
		return nil
	}
	return &AddToolchainResult{
		Name:      m.name,
		Runner:    m.runner,
		BuildType: m.buildType,
	}
}

func RunAddToolchainTUI(existingNames []string, runnerNames []string) (*AddToolchainResult, error) {
	m := NewAddToolchainModel(existingNames, runnerNames)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	return final.(AddToolchainModel).GetResult(), nil
}

// =========================================
// Add Runner TUI (execution environment + optional compiler settings)
// =========================================

type AddRunnerStep int

const (
	RunnerStepName AddRunnerStep = iota
	RunnerStepType
	RunnerStepDockerImage
	RunnerStepCheckingImage
	RunnerStepCompilerCC
	RunnerStepCompilerCXX
	RunnerStepCMakeToolchain
	RunnerStepSSHHost
	RunnerStepSSHUser
	RunnerStepDone
)

type AddRunnerModel struct {
	step             AddRunnerStep
	textInput        textinput.Model
	spinner          spinner.Model
	cursor           int
	quitting         bool
	cancelled        bool
	errorMsg         string
	checkingStatus   string
	existingNames    map[string]bool
	name             string
	runnerType       string
	image            string
	host             string
	user             string
	cc               string
	cxx              string
	cmakeToolchain   string
	typeOptions      []string
	availableImages  []DockerImage
	filteredImages   []DockerImage
	imageCursor      int
	imageScrollStart int
	maxVisibleImages int
}

type AddRunnerResult struct {
	Name           string
	Type           string
	Image          string
	Host           string
	User           string
	CC             string
	CXX            string
	CMakeToolchain string
}

func NewAddRunnerModel(existingNames []string) AddRunnerModel {
	ti := textinput.New()
	ti.Placeholder = "docker-gcc"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40
	ti.TextStyle = inputTextStyle

	existing := make(map[string]bool)
	for _, n := range existingNames {
		existing[n] = true
	}

	images := listDockerImages()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return AddRunnerModel{
		step:             RunnerStepName,
		textInput:        ti,
		spinner:          s,
		existingNames:    existing,
		typeOptions:      []string{"docker", "ssh"},
		availableImages:  images,
		filteredImages:   images,
		maxVisibleImages: 6,
	}
}

func (m AddRunnerModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m AddRunnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle spinner tick during checking
	if m.step == RunnerStepCheckingImage {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" || msg.String() == "esc" {
				m.quitting = true
				m.cancelled = true
				return m, tea.Quit
			}
			return m, nil
		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		case ImageCheckResult:
			if msg.Success {
				// Proceed to compiler settings
				m.step = RunnerStepCompilerCC
				m.textInput.Reset()
				m.textInput.Placeholder = "(optional, e.g. gcc-13)"
				m.textInput.Focus()
				return m, nil
			} else {
				m.errorMsg = msg.Error
				m.step = RunnerStepDockerImage
				m.textInput.Focus()
				return m, nil
			}
		}
		return m, nil
	}

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
			if m.step == RunnerStepType {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.typeOptions) - 1
				}
				return m, nil
			} else if m.step == RunnerStepDockerImage && len(m.filteredImages) > 0 {
				if m.imageCursor > 0 {
					m.imageCursor--
					if m.imageCursor < m.imageScrollStart {
						m.imageScrollStart = m.imageCursor
					}
				}
				return m, nil
			}
		case "down", "j":
			if m.step == RunnerStepType {
				m.cursor++
				if m.cursor >= len(m.typeOptions) {
					m.cursor = 0
				}
				return m, nil
			} else if m.step == RunnerStepDockerImage && len(m.filteredImages) > 0 {
				maxCursor := len(m.filteredImages) - 1
				if m.imageCursor < maxCursor {
					m.imageCursor++
					if m.imageCursor >= m.imageScrollStart+m.maxVisibleImages {
						m.imageScrollStart = m.imageCursor - m.maxVisibleImages + 1
					}
				}
				return m, nil
			}
		case "tab":
			if m.step == RunnerStepDockerImage && len(m.filteredImages) > 0 && m.imageCursor < len(m.filteredImages) {
				m.textInput.SetValue(m.filteredImages[m.imageCursor].FullName())
				return m, nil
			}
		}
	}

	// Update text input and filter images
	if m.step == RunnerStepDockerImage || m.step == RunnerStepName || m.step == RunnerStepSSHHost || m.step == RunnerStepSSHUser || m.step == RunnerStepCompilerCC || m.step == RunnerStepCompilerCXX || m.step == RunnerStepCMakeToolchain {
		var cmd tea.Cmd
		oldValue := m.textInput.Value()
		m.textInput, cmd = m.textInput.Update(msg)

		if m.step == RunnerStepDockerImage && m.textInput.Value() != oldValue {
			filter := strings.ToLower(m.textInput.Value())
			m.filteredImages = nil
			for _, img := range m.availableImages {
				if filter == "" || strings.Contains(strings.ToLower(img.FullName()), filter) {
					m.filteredImages = append(m.filteredImages, img)
				}
			}
			m.imageCursor = 0
			m.imageScrollStart = 0
		}
		return m, cmd
	}
	return m, nil
}

func (m AddRunnerModel) handleEnter() (tea.Model, tea.Cmd) {
	m.errorMsg = ""
	value := strings.TrimSpace(m.textInput.Value())

	switch m.step {
	case RunnerStepName:
		if value == "" {
			m.errorMsg = "Name is required"
			return m, nil
		}
		if m.existingNames[value] {
			m.errorMsg = fmt.Sprintf("Runner '%s' already exists", value)
			return m, nil
		}
		m.name = value
		m.step = RunnerStepType
		m.cursor = 0

	case RunnerStepType:
		m.runnerType = m.typeOptions[m.cursor]
		switch m.runnerType {
		case "docker":
			m.step = RunnerStepDockerImage
			m.textInput.Reset()
			m.textInput.Placeholder = "gcc:13"
			m.textInput.Focus()
		case "ssh":
			m.step = RunnerStepSSHHost
			m.textInput.Reset()
			m.textInput.Placeholder = "build-server.local"
			m.textInput.Focus()
		}

	case RunnerStepDockerImage:
		if len(m.filteredImages) > 0 && m.imageCursor < len(m.filteredImages) {
			m.image = m.filteredImages[m.imageCursor].FullName()
		} else if value != "" {
			m.image = value
		} else {
			m.errorMsg = "Docker image is required"
			return m, nil
		}
		m.step = RunnerStepCheckingImage
		m.checkingStatus = "Checking build tools..."
		return m, tea.Batch(m.spinner.Tick, checkImageToolsCmd(m.image))

	case RunnerStepCompilerCC:
		m.cc = value // Can be empty
		m.step = RunnerStepCompilerCXX
		m.textInput.Reset()
		if m.cc != "" {
			defaultCxx := strings.Replace(m.cc, "gcc", "g++", 1)
			defaultCxx = strings.Replace(defaultCxx, "clang", "clang++", 1)
			m.textInput.Placeholder = defaultCxx
		} else {
			m.textInput.Placeholder = "(optional, e.g. g++-13)"
		}
		m.textInput.Focus()

	case RunnerStepCompilerCXX:
		m.cxx = value // Can be empty
		m.step = RunnerStepCMakeToolchain
		m.textInput.Reset()
		m.textInput.Placeholder = "(optional)"
		m.textInput.Focus()

	case RunnerStepCMakeToolchain:
		m.cmakeToolchain = value
		m.step = RunnerStepDone
		m.quitting = true
		return m, tea.Quit

	case RunnerStepSSHHost:
		if value == "" {
			m.errorMsg = "SSH host is required"
			return m, nil
		}
		m.host = value
		m.step = RunnerStepSSHUser
		m.textInput.Reset()
		m.textInput.Placeholder = "(optional)"
		m.textInput.Focus()

	case RunnerStepSSHUser:
		m.user = value
		// SSH runners also go to compiler settings
		m.step = RunnerStepCompilerCC
		m.textInput.Reset()
		m.textInput.Placeholder = "(optional, e.g. gcc-13)"
		m.textInput.Focus()
	}

	return m, nil
}

func (m AddRunnerModel) View() string {
	if m.quitting && m.cancelled {
		return "\n  " + dimStyle.Render("Cancelled.") + "\n\n"
	}
	if m.step == RunnerStepDone {
		return ""
	}

	var s strings.Builder
	s.WriteString("\n")

	// Show answered questions
	if m.name != "" {
		s.WriteString("  " + successStyle.Render("✓") + " Runner name: " + m.name + "\n")
	}
	if m.runnerType != "" {
		s.WriteString("  " + successStyle.Render("✓") + " Runner type: " + m.runnerType + "\n")
	}
	if m.image != "" {
		s.WriteString("  " + successStyle.Render("✓") + " Docker image: " + m.image + "\n")
	}
	if m.host != "" {
		s.WriteString("  " + successStyle.Render("✓") + " SSH host: " + m.host + "\n")
	}
	if m.user != "" {
		s.WriteString("  " + successStyle.Render("✓") + " SSH user: " + m.user + "\n")
	}

	switch m.step {
	case RunnerStepName:
		s.WriteString("\n  " + questionStyle.Render("? Runner name") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")

	case RunnerStepType:
		s.WriteString("\n  " + questionStyle.Render("? Runner type") + "\n")
		for i, opt := range m.typeOptions {
			cursor := "  "
			if m.cursor == i {
				cursor = selectedStyle.Render("❯ ")
			}
			desc := ""
			if opt == "docker" {
				desc = dimStyle.Render(" - build in container")
			} else if opt == "ssh" {
				desc = dimStyle.Render(" - build on remote server")
			}
			s.WriteString("  " + cursor + opt + desc + "\n")
		}

	case RunnerStepDockerImage:
		s.WriteString("\n  " + questionStyle.Render("? Docker image") + " " + dimStyle.Render("(type to filter, ↑↓ to select, Tab to complete)") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")

		if len(m.filteredImages) > 0 {
			s.WriteString("\n")
			endIdx := m.imageScrollStart + m.maxVisibleImages
			if endIdx > len(m.filteredImages) {
				endIdx = len(m.filteredImages)
			}

			for i := m.imageScrollStart; i < endIdx; i++ {
				img := m.filteredImages[i]
				display := img.FullName()
				if img.Architecture != "" {
					display += " " + dimStyle.Render("("+img.Architecture+")")
				}

				cursor := "  "
				if m.imageCursor == i {
					cursor = selectedStyle.Render("❯ ")
					s.WriteString("  " + cursor + selectedStyle.Render(img.FullName()))
					if img.Architecture != "" {
						s.WriteString(" " + dimStyle.Render("("+img.Architecture+")"))
					}
					s.WriteString("\n")
				} else {
					s.WriteString("  " + cursor + dimStyle.Render(display) + "\n")
				}
			}

			if len(m.filteredImages) > m.maxVisibleImages {
				shown := fmt.Sprintf("(%d of %d)", endIdx-m.imageScrollStart, len(m.filteredImages))
				s.WriteString("  " + dimStyle.Render(shown) + "\n")
			}
		} else if len(m.availableImages) > 0 {
			s.WriteString("  " + dimStyle.Render("No matching images found") + "\n")
		}

	case RunnerStepCheckingImage:
		s.WriteString("\n  " + m.spinner.View() + " " + m.checkingStatus + "\n")

	case RunnerStepSSHHost:
		s.WriteString("\n  " + questionStyle.Render("? SSH host") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")

	case RunnerStepSSHUser:
		s.WriteString("\n  " + questionStyle.Render("? SSH user (optional)") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")

	case RunnerStepCompilerCC:
		s.WriteString("\n  " + questionStyle.Render("? C compiler") + " " + dimStyle.Render("(optional, leave blank for image default)") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")

	case RunnerStepCompilerCXX:
		if m.cc != "" {
			s.WriteString("  " + successStyle.Render("✓") + " C compiler: " + m.cc + "\n")
		}
		s.WriteString("\n  " + questionStyle.Render("? C++ compiler") + " " + dimStyle.Render("(optional)") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")

	case RunnerStepCMakeToolchain:
		if m.cc != "" {
			s.WriteString("  " + successStyle.Render("✓") + " C compiler: " + m.cc + "\n")
		}
		if m.cxx != "" {
			s.WriteString("  " + successStyle.Render("✓") + " C++ compiler: " + m.cxx + "\n")
		}
		s.WriteString("\n  " + questionStyle.Render("? CMake toolchain file") + " " + dimStyle.Render("(optional)") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")
	}

	if m.errorMsg != "" {
		wrapped := errorStyle.Width(70).Render("✗ " + m.errorMsg)
		s.WriteString("  " + wrapped + "\n")
	}

	if m.step != RunnerStepCheckingImage {
		s.WriteString("\n  " + dimStyle.Render("Enter to confirm • ↑↓ to select • Esc to cancel") + "\n")
	}
	return s.String()
}

func (m AddRunnerModel) GetResult() *AddRunnerResult {
	if m.cancelled {
		return nil
	}
	return &AddRunnerResult{
		Name:           m.name,
		Type:           m.runnerType,
		Image:          m.image,
		Host:           m.host,
		User:           m.user,
		CC:             m.cc,
		CXX:            m.cxx,
		CMakeToolchain: m.cmakeToolchain,
	}
}

func RunAddRunnerTUI(existingNames []string) (*AddRunnerResult, error) {
	m := NewAddRunnerModel(existingNames)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	return final.(AddRunnerModel).GetResult(), nil
}
