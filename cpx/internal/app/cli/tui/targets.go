package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Target represents a build target
type Target struct {
	Name     string
	Platform string // Human-readable platform description
}

// TargetState represents the current state of the target selection UI
type TargetState int

const (
	TargetStateSelecting TargetState = iota
	TargetStateDone
)

// TargetModel represents the target selection TUI state
type TargetModel struct {
	state    TargetState
	targets  []Target
	cursor   int
	selected map[int]bool
	quitting bool
	viewport int
	viewSize int
	Title    string // Custom title for the selection screen
}

// TargetResultMsg is returned when selection is complete
type TargetResultMsg struct {
	Selected []string
}

// NewTargetModel creates a new target selection model
func NewTargetModel(targets []Target, initialSelection []string, title string) TargetModel {
	if title == "" {
		title = "Select Build Targets"
	}

	selected := make(map[int]bool)
	targetMap := make(map[string]int)
	for i, t := range targets {
		targetMap[t.Name] = i
	}

	for _, name := range initialSelection {
		if idx, ok := targetMap[name]; ok {
			selected[idx] = true
		}
	}

	return TargetModel{
		state:    TargetStateSelecting,
		targets:  targets,
		selected: selected,
		viewSize: 15,
		Title:    title,
	}
}

// Init initializes the model
func (m TargetModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m TargetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			// If nothing selected, select current item
			if len(m.selected) == 0 {
				m.selected[m.cursor] = true
			}
			m.state = TargetStateDone
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.viewport {
					m.viewport = m.cursor
				}
			}

		case "down", "j":
			if m.cursor < len(m.targets)-1 {
				m.cursor++
				if m.cursor >= m.viewport+m.viewSize {
					m.viewport = m.cursor - m.viewSize + 1
				}
			}

		case " ":
			// Space to toggle selection
			m.selected[m.cursor] = !m.selected[m.cursor]
			if !m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			}

		case "tab":
			// Tab to select and move down
			m.selected[m.cursor] = true
			if m.cursor < len(m.targets)-1 {
				m.cursor++
				if m.cursor >= m.viewport+m.viewSize {
					m.viewport = m.cursor - m.viewSize + 1
				}
			} else if m.cursor < len(m.targets)-1 {
				m.cursor++
			}

		case "a":
			// 'a' to select all
			for i := range m.targets {
				m.selected[i] = true
			}

		case "n":
			// 'n' to clear selection
			m.selected = make(map[int]bool)
		}
	}

	return m, nil
}

// View renders the UI
func (m TargetModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	// Header
	s.WriteString(cyanBold.Render(m.Title) + "\n\n")

	if len(m.targets) == 0 {
		s.WriteString(dimStyle.Render("No targets available.\n"))
		return s.String()
	}

	// Results with viewport
	end := m.viewport + m.viewSize
	if end > len(m.targets) {
		end = len(m.targets)
	}

	// Show scroll indicator if needed
	if m.viewport > 0 {
		s.WriteString(dimStyle.Render("  ↑ more above\n"))
	}

	for i := m.viewport; i < end; i++ {
		target := m.targets[i]
		prefix := "  "
		style := lipgloss.NewStyle()

		if i == m.cursor {
			prefix = "▸ "
			style = selectedStyle
		}

		// Checkbox
		checkbox := "[ ]"
		if m.selected[i] {
			checkbox = greenCheck.Render("[✓]")
		}

		name := target.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}

		platform := target.Platform
		if len(platform) > 20 {
			platform = platform[:17] + "..."
		}

		line := fmt.Sprintf("%s%s %-20s %s", prefix, checkbox, name, dimStyle.Render(platform))
		if i == m.cursor {
			line = style.Render(fmt.Sprintf("%s%s %-20s", prefix, checkbox, name)) + " " + dimStyle.Render(platform)
		}
		s.WriteString(line + "\n")
	}

	// Show scroll indicator if needed
	if end < len(m.targets) {
		s.WriteString(dimStyle.Render("  ↓ more below\n"))
	}

	// Footer
	s.WriteString("\n")

	// Count selected
	selectedCount := len(m.selected)
	if selectedCount > 0 {
		s.WriteString(greenStyle.Render(fmt.Sprintf("%d selected", selectedCount)) + " • ")
	}

	s.WriteString(dimStyle.Render("Space: toggle • Tab: select & next • a: all • Enter: confirm • q: cancel"))

	return s.String()
}

// GetSelected returns the names of selected targets
func (m TargetModel) GetSelected() []string {
	var selected []string
	for i := range m.selected {
		selected = append(selected, m.targets[i].Name)
	}
	return selected
}

// RunTargetSelection runs the target selection TUI and returns selected targets
func RunTargetSelection(targets []Target, initialSelection []string, title string) ([]string, error) {
	m := NewTargetModel(targets, initialSelection, title)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	tm := finalModel.(TargetModel)
	if tm.quitting && tm.state != TargetStateDone {
		return nil, nil // User cancelled
	}

	return tm.GetSelected(), nil
}
