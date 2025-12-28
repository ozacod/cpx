package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SearchResult struct {
	Name        string
	Version     string
	Description string
}

// SearchState represents the current state of the search UI
type SearchState int

const (
	SearchStateInput SearchState = iota
	SearchStateSearching
	SearchStateResults
	SearchStateAdding
	SearchStateDone
)

// SearchFunc is a function that searches for packages
type SearchFunc func(query string) ([]SearchResult, error)

// AddFunc is a function that adds a package
type AddFunc func(pkg string) error

// SearchModel represents the search TUI state
type SearchModel struct {
	state          SearchState
	textInput      textinput.Model
	spinner        spinner.Model
	query          string
	results        []SearchResult
	cursor         int
	selected       map[int]bool
	err            error
	quitting       bool
	searchFunc     SearchFunc
	addFunc        AddFunc
	addedPackages  []string
	failedPackages map[string]string // package -> error message
	viewport       int               // For scrolling through results
	viewportSize   int
	currentPackage string   // Package currently being added
	addOutput      []string // Output from add operation
}

// SearchResultsMsg contains search results
type SearchResultsMsg struct {
	Results []SearchResult
	Err     error
}

// AddResultMsg contains add result
type AddResultMsg struct {
	Package string
	Success bool
	Err     error
}

// NewSearchModel creates a new search model
func NewSearchModel(initialQuery string, searchFunc SearchFunc, addFunc AddFunc) SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Enter package name to search..."
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40
	ti.PromptStyle = inputPromptStyle
	ti.TextStyle = inputTextStyle
	ti.Cursor.Style = cursorStyle
	ti.SetValue(initialQuery)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	m := SearchModel{
		state:          SearchStateInput,
		textInput:      ti,
		spinner:        s,
		selected:       make(map[int]bool),
		failedPackages: make(map[string]string),
		searchFunc:     searchFunc,
		addFunc:        addFunc,
		viewportSize:   15,
		addOutput:      []string{},
	}

	// If initial query provided, start searching immediately
	if initialQuery != "" {
		m.query = initialQuery
		m.state = SearchStateSearching
	}

	return m
}

// Init initializes the model
func (m SearchModel) Init() tea.Cmd {
	if m.state == SearchStateSearching {
		return tea.Batch(m.spinner.Tick, m.doSearch())
	}
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m SearchModel) doSearch() tea.Cmd {
	return func() tea.Msg {
		results, err := m.searchFunc(m.query)
		if err != nil {
			return SearchResultsMsg{Err: err}
		}
		return SearchResultsMsg{Results: results}
	}
}

func (m SearchModel) doAddPackage(pkg string) tea.Cmd {
	return func() tea.Msg {
		err := m.addFunc(pkg)
		if err != nil {
			return AddResultMsg{Package: pkg, Success: false, Err: err}
		}
		return AddResultMsg{Package: pkg, Success: true, Err: nil}
	}
}

// Update handles messages and updates the model
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.state == SearchStateResults && len(m.selected) > 0 {
				// Clear selection instead of quitting
				m.selected = make(map[int]bool)
				return m, nil
			}
			if m.state == SearchStateResults {
				// Go back to input
				m.state = SearchStateInput
				m.textInput.Focus()
				return m, textinput.Blink
			}
			m.quitting = true
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.state == SearchStateResults && m.cursor > 0 {
				m.cursor--
				// Scroll viewport if needed
				if m.cursor < m.viewport {
					m.viewport = m.cursor
				}
			}

		case "down", "j":
			if m.state == SearchStateResults && m.cursor < len(m.results)-1 {
				m.cursor++
				// Scroll viewport if needed
				if m.cursor >= m.viewport+m.viewportSize {
					m.viewport = m.cursor - m.viewportSize + 1
				}
			}

		case " ":
			// Space to toggle selection
			if m.state == SearchStateResults {
				m.selected[m.cursor] = !m.selected[m.cursor]
				if !m.selected[m.cursor] {
					delete(m.selected, m.cursor)
				}
			}

		case "tab":
			// Tab to select and move down
			if m.state == SearchStateResults {
				m.selected[m.cursor] = true
				if m.cursor < len(m.results)-1 {
					m.cursor++
					if m.cursor >= m.viewport+m.viewportSize {
						m.viewport = m.cursor - m.viewportSize + 1
					}
				}
			}

		case "a":
			// 'a' to select all visible
			if m.state == SearchStateResults {
				for i := range m.results {
					m.selected[i] = true
				}
			}
		}

	case SearchResultsMsg:
		if msg.Err != nil {
			m.err = msg.Err
			m.quitting = true
			return m, tea.Quit
		}
		m.results = msg.Results
		m.state = SearchStateResults
		m.cursor = 0
		m.viewport = 0
		return m, nil

	case AddResultMsg:
		if msg.Success {
			m.addedPackages = append(m.addedPackages, msg.Package)
		} else if msg.Err != nil {
			// Track failed packages with their error
			m.failedPackages[msg.Package] = msg.Err.Error()
		}
		// Check if there are more packages to add
		for idx := range m.selected {
			pkgName := m.results[idx].Name
			if !contains(m.addedPackages, pkgName) && m.failedPackages[pkgName] == "" {
				m.currentPackage = pkgName
				return m, tea.Batch(m.spinner.Tick, m.doAddPackage(pkgName))
			}
		}
		// All done
		m.state = SearchStateDone
		return m, tea.Quit

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Handle text input
	if m.state == SearchStateInput {
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (m SearchModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case SearchStateInput:
		query := strings.TrimSpace(m.textInput.Value())
		if query == "" {
			return m, nil
		}
		m.query = query
		m.state = SearchStateSearching
		return m, tea.Batch(m.spinner.Tick, m.doSearch())

	case SearchStateResults:
		if len(m.selected) == 0 {
			// If nothing selected, select current item
			m.selected[m.cursor] = true
		}
		// Start adding packages
		m.state = SearchStateAdding
		m.addOutput = []string{}
		// Find first package to add
		for idx := range m.selected {
			m.currentPackage = m.results[idx].Name
			return m, tea.Batch(m.spinner.Tick, m.doAddPackage(m.results[idx].Name))
		}
	}

	return m, nil
}

// View renders the UI
func (m SearchModel) View() string {
	if m.quitting {
		if m.err != nil {
			return errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
		}
		return ""
	}

	var s strings.Builder

	switch m.state {
	case SearchStateInput:
		s.WriteString(cyanBold.Render("Search for packages") + "\n\n")
		s.WriteString(m.textInput.View() + "\n\n")
		s.WriteString(dimStyle.Render("Press Enter to search, Esc to quit"))

	case SearchStateSearching:
		s.WriteString(fmt.Sprintf("%s Searching for '%s'...\n", m.spinner.View(), m.query))

	case SearchStateResults:
		s.WriteString(m.renderResults())

	case SearchStateAdding:
		s.WriteString(fmt.Sprintf("%s Adding packages...\n", m.spinner.View()))
		for _, pkg := range m.addedPackages {
			s.WriteString(greenCheck.Render("✓") + " Added " + pkg + "\n")
		}
		for pkg, err := range m.failedPackages {
			s.WriteString(errorStyle.Render("✗") + " Failed to add " + pkg + ": " + err + "\n")
		}

	case SearchStateDone:
		s.WriteString(greenCheck.Render("✓") + " Done!\n\n")
		for _, pkg := range m.addedPackages {
			s.WriteString("  • " + pkg + "\n")
		}
		for pkg, err := range m.failedPackages {
			s.WriteString(errorStyle.Render("✗") + " Failed to add " + pkg + ": " + err + "\n")
		}
		if len(m.addedPackages) > 0 {
			s.WriteString("\n" + cyanBold.Render("📦 Find sample usage and more info at:") + "\n")
			for _, pkg := range m.addedPackages {
				s.WriteString("   https://cpx-dev.vercel.app/packages#package/" + pkg + "\n")
			}
		}
	}

	return s.String()
}

func (m SearchModel) renderResults() string {
	var s strings.Builder

	// Header
	s.WriteString(cyanBold.Render(fmt.Sprintf("Search results for '%s'", m.query)))
	s.WriteString(dimStyle.Render(fmt.Sprintf(" (%d found)", len(m.results))))
	s.WriteString("\n\n")

	if len(m.results) == 0 {
		s.WriteString(dimStyle.Render("No packages found.\n"))
		s.WriteString("\n" + dimStyle.Render("Press Esc to go back"))
		return s.String()
	}

	// Results with viewport
	end := m.viewport + m.viewportSize
	if end > len(m.results) {
		end = len(m.results)
	}

	// Show scroll indicator if needed
	if m.viewport > 0 {
		s.WriteString(dimStyle.Render("  ↑ more above\n"))
	}

	for i := m.viewport; i < end; i++ {
		result := m.results[i]
		prefix := "  "
		style := lipgloss.NewStyle()

		if i == m.cursor {
			prefix = "▸ "
			style = selectedStyle
		}

		checkbox := "[ ]"
		if m.selected[i] {
			checkbox = greenCheck.Render("[✓]")
		}

		name := result.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		desc := result.Description
		if len(desc) > 45 {
			desc = desc[:42] + "..."
		}

		line := fmt.Sprintf("%s%s %-30s %s", prefix, checkbox, name, dimStyle.Render(desc))
		if i == m.cursor {
			line = style.Render(fmt.Sprintf("%s%s %-30s", prefix, checkbox, name)) + " " + dimStyle.Render(desc)
		}
		s.WriteString(line + "\n")
	}

	// Show scroll indicator if needed
	if end < len(m.results) {
		s.WriteString(dimStyle.Render("  ↓ more below\n"))
	}

	// Footer
	s.WriteString("\n")
	selectedCount := len(m.selected)
	if selectedCount > 0 {
		s.WriteString(greenStyle.Render(fmt.Sprintf("%d selected", selectedCount)) + " • ")
	}
	s.WriteString(dimStyle.Render("Space: toggle • Tab: select & next • Enter: add selected • Esc: back"))

	return s.String()
}

// RunSearch runs the search TUI and returns selected packages
func RunSearch(initialQuery string, searchFunc SearchFunc, addFunc AddFunc) error {
	m := NewSearchModel(initialQuery, searchFunc, addFunc)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
