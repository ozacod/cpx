package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// =========================================
// Add Runner TUI (execution environment + optional compiler settings)
// =========================================

type AddRunnerStep int

const (
	RunnerStepName AddRunnerStep = iota
	RunnerStepType
	RunnerStepDockerImage
	RunnerStepCheckingImage
	RunnerStepSSHHost
	RunnerStepSSHUser
	RunnerStepDone
)

type ImagesLoadedMsg []DockerImage

type AddRunnerModel struct {
	step      AddRunnerStep
	textInput textinput.Model
	spinner   spinner.Model
	cursor    int
	err       error
	errorMsg  string // Displayed error message
	cancelled bool

	// Data
	name       string
	runnerType string
	image      string
	sshHost    string
	sshUser    string

	// Options
	existingNames map[string]bool
	typeOptions   []string // docker, ssh

	// Docker specific
	availableImages  []DockerImage
	filteredImages   []DockerImage
	imageFilter      string
	imageScrollStart int
	maxVisibleImages int

	currentQuestion string
}

type AddRunnerResult struct {
	Name    string
	Type    string
	Image   string
	SSHHost string
	SSHUser string
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

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return AddRunnerModel{
		step:             RunnerStepName,
		textInput:        ti,
		spinner:          s,
		existingNames:    existing,
		typeOptions:      []string{"docker", "ssh"},
		availableImages:  nil, // Loaded async
		filteredImages:   nil,
		maxVisibleImages: 6,
	}
}

func (m AddRunnerModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick, func() tea.Msg {
		return ImagesLoadedMsg(listDockerImages())
	})
}

func (m AddRunnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ImagesLoadedMsg:
		m.availableImages = msg
		m.filteredImages = msg
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "up", "down":
			if m.step == RunnerStepDockerImage {
				m.handleImageListNav(msg.String())
				return m, nil
			} else if m.step == RunnerStepType {
				if msg.String() == "up" && m.cursor > 0 {
					m.cursor--
				} else if msg.String() == "down" && m.cursor < len(m.typeOptions)-1 {
					m.cursor++
				}
				return m, nil
			}
		}
	case ImageCheckResult:
		if msg.Success {
			// Proceed to Done directly
			m.step = RunnerStepDone
			return m, tea.Quit
		} else {
			m.errorMsg = msg.Error
			m.step = RunnerStepDockerImage
			m.textInput.Focus()
			return m, nil
		}
	}

	var cmd tea.Cmd
	// Only update text input if not in selection modes (except for filtering images)
	if m.step == RunnerStepName || m.step == RunnerStepDockerImage || m.step == RunnerStepSSHHost || m.step == RunnerStepSSHUser {
		m.textInput, cmd = m.textInput.Update(msg)

		if m.step == RunnerStepDockerImage {
			m.imageFilter = m.textInput.Value()
			m.filterImages()
		}
	}

	// Spinner tick
	var sCmd tea.Cmd
	if m.step == RunnerStepCheckingImage {
		m.spinner, sCmd = m.spinner.Update(msg)
	}

	return m, tea.Batch(cmd, sCmd)
}

func (m AddRunnerModel) handleEnter() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.textInput.Value())

	switch m.step {
	case RunnerStepName:
		if val == "" {
			m.errorMsg = "Name cannot be empty"
			return m, nil
		}
		if m.existingNames[val] {
			m.errorMsg = "Runner with this name already exists"
			return m, nil
		}
		m.name = val
		m.step = RunnerStepType
		m.textInput.Reset()
		m.errorMsg = ""
		return m, nil

	case RunnerStepType:
		m.runnerType = m.typeOptions[m.cursor]
		if m.runnerType == "docker" {
			m.step = RunnerStepDockerImage
			m.textInput.Reset()
			m.textInput.Placeholder = "Filter images..."
			m.errorMsg = ""
			m.filterImages()
		} else {
			m.step = RunnerStepSSHHost
			m.textInput.Reset()
			m.textInput.Placeholder = "user@hostname or hostname"
			m.textInput.Focus()
		}
		return m, nil

	case RunnerStepDockerImage:
		// User selected an image from list or typed one?
		if len(m.filteredImages) > 0 {
			m.image = m.filteredImages[m.cursor].Repository + ":" + m.filteredImages[m.cursor].Tag
			m.step = RunnerStepCheckingImage
			return m, checkImageToolsCmd(m.image)
		}
		if val != "" {
			m.image = val
			m.step = RunnerStepCheckingImage
			return m, checkImageToolsCmd(m.image)
		}
		return m, nil

	case RunnerStepSSHHost:
		if val == "" {
			m.errorMsg = "Host cannot be empty"
			return m, nil
		}
		m.sshHost = val
		m.step = RunnerStepSSHUser
		m.textInput.Reset()
		m.textInput.Placeholder = "username"
		return m, nil

	case RunnerStepSSHUser:
		m.sshUser = val
		m.step = RunnerStepDone
		return m, tea.Quit
	}

	return m, nil
}

func (m *AddRunnerModel) filterImages() {
	if m.imageFilter == "" {
		m.filteredImages = m.availableImages
		return
	}

	var filtered []DockerImage
	for _, img := range m.availableImages {
		full := img.Repository + ":" + img.Tag
		if strings.Contains(strings.ToLower(full), strings.ToLower(m.imageFilter)) {
			filtered = append(filtered, img)
		}
	}
	m.filteredImages = filtered
	if m.cursor >= len(m.filteredImages) {
		m.cursor = 0
	}
}

func (m *AddRunnerModel) handleImageListNav(key string) {
	if len(m.filteredImages) == 0 {
		return
	}
	if key == "up" {
		if m.cursor > 0 {
			m.cursor--
		}
		if m.cursor < m.imageScrollStart {
			m.imageScrollStart--
		}
	} else if key == "down" {
		if m.cursor < len(m.filteredImages)-1 {
			m.cursor++
		}
		if m.cursor >= m.imageScrollStart+m.maxVisibleImages {
			m.imageScrollStart++
		}
	}
}

func (m AddRunnerModel) View() string {
	var s strings.Builder

	// Header - removed as per user request to remove purple header and whitespace
	// s.WriteString(titleStyle.Render("Add New Runner") + "\n\n")

	if m.step == RunnerStepDone {
		return ""
	}

	if m.errorMsg != "" {
		s.WriteString(errorStyle.Render("Error: "+m.errorMsg) + "\n\n")
	}

	switch m.step {
	case RunnerStepName:
		s.WriteString("\n  " + questionStyle.Render("? Runner name") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")
		s.WriteString("\n" + helpStyle.Render("Enter a unique name for this runner"))

	case RunnerStepType:
		s.WriteString("\n  " + questionStyle.Render("? Runner type") + "\n\n")
		for i, opt := range m.typeOptions {
			cursor := " "
			style := textStyle
			if m.cursor == i {
				cursor = ">"
				style = selectedStyle
			}
			s.WriteString(fmt.Sprintf("  %s %s\n", cursor, style.Render(opt)))
		}

	case RunnerStepDockerImage:
		s.WriteString("\n  " + questionStyle.Render("? Select Docker image") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n\n")

		if len(m.filteredImages) == 0 {
			s.WriteString("  " + helpStyle.Render("No images found matching filter") + "\n")
		} else {
			end := m.imageScrollStart + m.maxVisibleImages
			if end > len(m.filteredImages) {
				end = len(m.filteredImages)
			}

			for i := m.imageScrollStart; i < end; i++ {
				img := m.filteredImages[i]
				cursor := " "
				style := textStyle
				if m.cursor == i {
					cursor = ">"
					style = selectedStyle
				}
				s.WriteString(fmt.Sprintf("  %s %s (%s)\n", cursor, style.Render(img.Repository+":"+img.Tag), img.Size))
			}
		}

	case RunnerStepCheckingImage:
		s.WriteString(fmt.Sprintf("\n  %s Checking image tools...\n", m.spinner.View()))

	case RunnerStepSSHHost:
		s.WriteString("\n  " + questionStyle.Render("? SSH Host") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")

	case RunnerStepSSHUser:
		s.WriteString("\n  " + questionStyle.Render("? SSH User") + "\n")
		s.WriteString("  " + m.textInput.View() + "\n")
	}

	s.WriteString("\n\n" + helpStyle.Render("Press Esc to cancel, Enter to confirm"))
	return s.String()
}

func (m AddRunnerModel) GetResult() *AddRunnerResult {
	if m.cancelled {
		return nil
	}
	return &AddRunnerResult{
		Name:    m.name,
		Type:    m.runnerType,
		Image:   m.image,
		SSHHost: m.sshHost,
		SSHUser: m.sshUser,
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
