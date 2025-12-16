package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ToolchainItem represents a build toolchain for selection
type ToolchainItem struct {
	Name     string
	Platform string // Human-readable platform description
}

// ToolchainListState represents the current state of the list UI
type ToolchainListState int

const (
	ToolchainListStateSelecting ToolchainListState = iota
	ToolchainListStateDone
)

// ToolchainListModel represents the toolchain selection TUI state
type ToolchainListModel struct {
	state    ToolchainListState
	items    []ToolchainItem
	cursor   int
	selected map[int]bool
	quitting bool
	viewport int
	viewSize int
	Title    string // Custom title for the selection screen
}

// ToolchainListResultMsg is returned when selection is complete
type ToolchainListResultMsg struct {
	Selected []string
}

// NewToolchainListModel creates a new toolchain selection model
func NewToolchainListModel(items []ToolchainItem, initialSelection []string, title string) ToolchainListModel {
	if title == "" {
		title = "Select Toolchains"
	}

	selected := make(map[int]bool)
	itemMap := make(map[string]int)
	for i, t := range items {
		itemMap[t.Name] = i
	}

	for _, name := range initialSelection {
		if idx, ok := itemMap[name]; ok {
			selected[idx] = true
		}
	}

	return ToolchainListModel{
		state:    ToolchainListStateSelecting,
		items:    items,
		selected: selected,
		viewSize: 15,
		Title:    title,
	}
}

// Init initializes the model
func (m ToolchainListModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m ToolchainListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.state = ToolchainListStateDone
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.viewport {
					m.viewport = m.cursor
				}
			}

		case "down", "j":
			if m.cursor < len(m.items)-1 {
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
			if m.cursor < len(m.items)-1 {
				m.cursor++
				if m.cursor >= m.viewport+m.viewSize {
					m.viewport = m.cursor - m.viewSize + 1
				}
			} else if m.cursor < len(m.items)-1 {
				m.cursor++
			}

		case "a":
			// 'a' to select all
			for i := range m.items {
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
func (m ToolchainListModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	// Header
	s.WriteString(cyanBold.Render(m.Title) + "\n\n")

	if len(m.items) == 0 {
		s.WriteString(dimStyle.Render("No toolchains available.\n"))
		return s.String()
	}

	// Results with viewport
	end := m.viewport + m.viewSize
	if end > len(m.items) {
		end = len(m.items)
	}

	// Show scroll indicator if needed
	if m.viewport > 0 {
		s.WriteString(dimStyle.Render("  ↑ more above\n"))
	}

	for i := m.viewport; i < end; i++ {
		item := m.items[i]
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

		name := item.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}

		platform := item.Platform
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
	if end < len(m.items) {
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

// GetSelected returns the names of selected toolchains
func (m ToolchainListModel) GetSelected() []string {
	var selected []string
	for i := range m.selected {
		selected = append(selected, m.items[i].Name)
	}
	return selected
}

// RunToolchainSelection runs the selection TUI and returns selected names
func RunToolchainSelection(items []ToolchainItem, initialSelection []string, title string) ([]string, error) {
	m := NewToolchainListModel(items, initialSelection, title)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	tm := finalModel.(ToolchainListModel)
	if tm.quitting && tm.state != ToolchainListStateDone {
		return nil, nil // User cancelled
	}

	return tm.GetSelected(), nil
}
