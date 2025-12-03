package cmd

import (
	"fmt"
	"os"
	"strings"
)

// Colors for terminal output
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
)

// Icon constants for consistent output
const (
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "⚠"
	IconInfo    = "ℹ"
	IconArrow   = "→"
	IconBuild   = "🔨"
	IconRun     = "▶"
	IconTest    = "🧪"
	IconPackage = "📦"
)

// Version is the cpx version
const Version = "1.0.5"

// DefaultServer is the default server URL
const DefaultServer = "https://cpxcpp.vercel.app"

// DefaultCfgFile is the default config file name
const DefaultCfgFile = "cpx.yaml"

// LockFile is the lock file name
const LockFile = "cpx.lock"

// PrintSuccess prints a success message
func PrintSuccess(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s%s %s%s\n", Green, IconSuccess, msg, Reset)
}

// PrintError prints an error message
func PrintError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s%s %s%s\n", Red, IconError, msg, Reset)
}

// PrintWarning prints a warning message
func PrintWarning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s%s %s%s\n", Yellow, IconWarning, msg, Reset)
}

// PrintInfo prints an info message
func PrintInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s%s %s%s\n", Cyan, IconInfo, msg, Reset)
}

// PrintStep prints a step in a process
func PrintStep(step, total int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%d/%d]%s %s\n", Dim, step, total, Reset, msg)
}

// PrintCommand prints a command that will be executed
func PrintCommand(cmd string) {
	fmt.Printf("%s%s %s%s\n", Dim, IconArrow, cmd, Reset)
}

// PrintHeader prints a section header
func PrintHeader(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("\n%s%s%s%s\n", Bold, Cyan, msg, Reset)
	fmt.Println(strings.Repeat("─", len(msg)))
}

// ExitWithError prints an error message and exits with status 1
func ExitWithError(err error) {
	PrintError("%v", err)
	os.Exit(1)
}

// Spinner represents a simple progress spinner
type Spinner struct {
	frames  []string
	current int
	message string
}

// NewSpinner creates a new spinner with a message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		current: 0,
		message: message,
	}
}

// Tick advances the spinner and prints the current frame
func (s *Spinner) Tick() {
	fmt.Printf("\r%s%s%s %s", Cyan, s.frames[s.current], Reset, s.message)
	s.current = (s.current + 1) % len(s.frames)
}

// Done finishes the spinner with a success message
func (s *Spinner) Done(message string) {
	fmt.Printf("\r%s%s %s%s\n", Green, IconSuccess, message, Reset)
}

// Fail finishes the spinner with an error message
func (s *Spinner) Fail(message string) {
	fmt.Printf("\r%s%s %s%s\n", Red, IconError, message, Reset)
}
