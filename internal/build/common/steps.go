package common

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ozacod/cpx/internal/utils/colors"
)

type StepState int

const (
	StepPending StepState = iota
	StepRunning
	StepDone
	StepFailed
)

type Step struct {
	Name          string
	State         StepState
	Progress      int // 0-100 for running steps
	Duration      time.Duration
	Detail        string // e.g., "src/utils/parser.cpp"
	Indeterminate bool   // true for steps without parsable progress (e.g., configure)
}

type StepProgress struct {
	mu           sync.Mutex
	projectName  string
	buildType    string
	optLabel     string
	steps        []*Step
	currentStep  int
	startTime    time.Time
	stepStart    time.Time
	verbose      bool
	lineCount    int // number of lines we've printed (for clearing)
	spinnerFrame int
	stopCh       chan struct{} // channel to stop the spinner goroutine
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	barFilled = "━"
	barEmpty  = "░"
	barWidth  = 20
)

// NewStepProgress creates a new step progress tracker
func NewStepProgress(projectName, buildType, optLabel string, stepNames []string, verbose bool) *StepProgress {
	steps := make([]*Step, len(stepNames))
	for i, name := range stepNames {
		steps[i] = &Step{
			Name:  name,
			State: StepPending,
		}
	}

	return &StepProgress{
		projectName: projectName,
		buildType:   buildType,
		optLabel:    optLabel,
		steps:       steps,
		currentStep: -1,
		startTime:   time.Now(),
		verbose:     verbose,
	}
}

// Start begins the progress display and starts spinner animation
func (sp *StepProgress) Start() {
	if sp.verbose {
		return
	}

	sp.stopCh = make(chan struct{})
	sp.render()

	// Start spinner animation goroutine
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-sp.stopCh:
				return
			case <-ticker.C:
				sp.mu.Lock()
				sp.spinnerFrame = (sp.spinnerFrame + 1) % len(spinnerFrames)
				sp.render()
				sp.mu.Unlock()
			}
		}
	}()
}

// StartStep marks a step as running
func (sp *StepProgress) StartStep(index int) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if index < 0 || index >= len(sp.steps) {
		return
	}

	sp.currentStep = index
	sp.steps[index].State = StepRunning
	sp.steps[index].Progress = 0
	sp.stepStart = time.Now()

	if !sp.verbose {
		sp.render()
	} else {
		fmt.Printf("%s  • %s%s\n", colors.Cyan, sp.steps[index].Name, colors.Reset)
	}
}

// SetIndeterminate marks a step as having no parsable progress (shows spinner only)
func (sp *StepProgress) SetIndeterminate(index int, indeterminate bool) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if index < 0 || index >= len(sp.steps) {
		return
	}

	sp.steps[index].Indeterminate = indeterminate
}

// UpdateProgress updates the progress of the current step
func (sp *StepProgress) UpdateProgress(progress int, detail string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.currentStep < 0 || sp.currentStep >= len(sp.steps) {
		return
	}

	sp.steps[sp.currentStep].Progress = progress
	if detail != "" {
		sp.steps[sp.currentStep].Detail = detail
	}
	// Note: render() is called by the background ticker, no need to call it here
}

// CompleteStep marks the current step as done
func (sp *StepProgress) CompleteStep(index int) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if index < 0 || index >= len(sp.steps) {
		return
	}

	sp.steps[index].State = StepDone
	sp.steps[index].Progress = 100
	sp.steps[index].Duration = time.Since(sp.stepStart)
	sp.steps[index].Detail = ""

	if !sp.verbose {
		sp.render()
	} else {
		fmt.Printf("%s  ✓ %s complete (%s)%s\n",
			colors.Green, sp.steps[index].Name, formatDuration(sp.steps[index].Duration), colors.Reset)
	}
}

// FailStep marks a step as failed
func (sp *StepProgress) FailStep(index int) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if index < 0 || index >= len(sp.steps) {
		return
	}

	sp.steps[index].State = StepFailed
	sp.steps[index].Duration = time.Since(sp.stepStart)

	if !sp.verbose {
		sp.render()
	}
}

// Finish completes the progress display
func (sp *StepProgress) Finish(success bool) {
	// Stop the spinner goroutine first (outside lock to avoid deadlock)
	if sp.stopCh != nil {
		close(sp.stopCh)
		sp.stopCh = nil
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.verbose {
		return
	}

	// Clear the display and print final state
	sp.clearLines()
	sp.printFinalState(success)
}

// render draws the current state to the terminal
func (sp *StepProgress) render() {
	// Clear previous output
	sp.clearLines()

	var sb strings.Builder

	// Header: ▸ Build myproject (debug) [opt: -O0]
	sb.WriteString(fmt.Sprintf("\n%s▸ Build%s %s %s(%s)%s %s[opt: %s]%s\n\n",
		colors.Cyan, colors.Reset, sp.projectName,
		colors.Gray, sp.buildType, colors.Reset,
		colors.Gray, sp.optLabel, colors.Reset))

	// Steps
	for i, step := range sp.steps {
		sb.WriteString(sp.renderStep(i, step))
		sb.WriteString("\n")
	}

	// Current step detail (e.g., "compiling: src/main.cpp")
	if sp.currentStep >= 0 && sp.currentStep < len(sp.steps) {
		step := sp.steps[sp.currentStep]
		if step.Detail != "" && step.State == StepRunning {
			sb.WriteString(fmt.Sprintf("\n  %scompiling:%s %s\n",
				colors.Gray, colors.Reset, step.Detail))
		}
	}

	output := sb.String()
	fmt.Fprint(os.Stderr, output)

	// Count lines for clearing next time
	sp.lineCount = strings.Count(output, "\n")
}

// renderStep renders a single step line
func (sp *StepProgress) renderStep(index int, step *Step) string {
	var icon, status, bar string

	switch step.State {
	case StepPending:
		icon = fmt.Sprintf("%s○%s", colors.Gray, colors.Reset)
		status = fmt.Sprintf("%spending%s", colors.Gray, colors.Reset)
		bar = sp.renderProgressBar(0, false)

	case StepRunning:
		spinner := spinnerFrames[sp.spinnerFrame]
		icon = fmt.Sprintf("%s%s%s", colors.Cyan, spinner, colors.Reset)
		if step.Indeterminate {
			// No progress bar for indeterminate steps
			bar = strings.Repeat(" ", barWidth)
			status = fmt.Sprintf("%s← running%s", colors.Gray, colors.Reset)
		} else {
			status = fmt.Sprintf("%s%3d%%%s", colors.Cyan, step.Progress, colors.Reset)
			bar = sp.renderProgressBar(step.Progress, true)
			status += fmt.Sprintf(" %s← current%s", colors.Gray, colors.Reset)
		}

	case StepDone:
		icon = fmt.Sprintf("%s✓%s", colors.Green, colors.Reset)
		status = fmt.Sprintf("%sdone%s (%s)", colors.Green, colors.Reset, formatDuration(step.Duration))
		if step.Indeterminate {
			bar = strings.Repeat(" ", barWidth)
		} else {
			bar = sp.renderProgressBar(100, false)
		}

	case StepFailed:
		icon = fmt.Sprintf("%s✗%s", colors.Red, colors.Reset)
		status = fmt.Sprintf("%sfailed%s", colors.Red, colors.Reset)
		bar = sp.renderProgressBar(step.Progress, false)
	}

	// Pad step name to align columns
	paddedName := fmt.Sprintf("%-14s", step.Name)

	return fmt.Sprintf("  %s %s%s %s", icon, paddedName, bar, status)
}

// renderProgressBar creates a progress bar string
func (sp *StepProgress) renderProgressBar(progress int, active bool) string {
	filled := (progress * barWidth) / 100
	if filled > barWidth {
		filled = barWidth
	}

	var sb strings.Builder

	if active {
		sb.WriteString(colors.Cyan)
	} else if progress == 100 {
		sb.WriteString(colors.Green)
	} else {
		sb.WriteString(colors.Gray)
	}

	for i := 0; i < filled; i++ {
		sb.WriteString(barFilled)
	}

	sb.WriteString(colors.Reset)
	sb.WriteString(colors.Gray)

	for i := filled; i < barWidth; i++ {
		sb.WriteString(barEmpty)
	}

	sb.WriteString(colors.Reset)

	return sb.String()
}

// clearLines clears the previously rendered lines
func (sp *StepProgress) clearLines() {
	for i := 0; i < sp.lineCount; i++ {
		// Move up and clear line
		fmt.Fprint(os.Stderr, "\033[A\033[2K")
	}
}

// printFinalState prints the final summary
func (sp *StepProgress) printFinalState(success bool) {
	fmt.Fprintf(os.Stderr, "\n%s▸ Build%s %s %s(%s)%s %s[opt: %s]%s\n\n",
		colors.Cyan, colors.Reset, sp.projectName,
		colors.Gray, sp.buildType, colors.Reset,
		colors.Gray, sp.optLabel, colors.Reset)

	for _, step := range sp.steps {
		var icon, status, bar string

		// Determine bar based on indeterminate flag
		if step.Indeterminate {
			bar = strings.Repeat(" ", barWidth)
		} else {
			bar = sp.renderProgressBar(100, false)
		}

		switch step.State {
		case StepDone:
			icon = fmt.Sprintf("%s✓%s", colors.Green, colors.Reset)
			status = fmt.Sprintf("%sdone%s (%s)", colors.Green, colors.Reset, formatDuration(step.Duration))
		case StepFailed:
			icon = fmt.Sprintf("%s✗%s", colors.Red, colors.Reset)
			if !step.Indeterminate {
				bar = sp.renderProgressBar(step.Progress, false)
			}
			status = fmt.Sprintf("%sfailed%s", colors.Red, colors.Reset)
		default:
			icon = fmt.Sprintf("%s○%s", colors.Gray, colors.Reset)
			if !step.Indeterminate {
				bar = sp.renderProgressBar(0, false)
			}
			status = fmt.Sprintf("%sskipped%s", colors.Gray, colors.Reset)
		}

		paddedName := fmt.Sprintf("%-14s", step.Name)
		fmt.Fprintf(os.Stderr, "  %s %s%s %s\n", icon, paddedName, bar, status)
	}

	fmt.Fprintln(os.Stderr)
}

// formatDuration formats a duration nicely
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}
