package cli

import (
	"fmt"
	"os"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	Bold   = "\033[1m"
)

// Icon constants for consistent output
const (
	IconSuccess = "✓"
	IconError   = "✗"
)

// Version is the cpx version
const Version = "1.1.5"

// DefaultServer is the default server URL
const DefaultServer = "https://cpx-dev.vercel.app"

// CpxConfig represents the project configuration for code generation
type CpxConfig struct {
	Package struct {
		Name        string
		Version     string
		CppStandard int
		Authors     []string
		Description string
	}
	Build struct {
		SharedLibs  bool
		ClangFormat string
		BuildType   string
		CxxFlags    string
	}
	VCS struct {
		Type string
	}
	PackageManager struct {
		Type string
	}
	Testing struct {
		Framework string
	}
	Hooks struct {
		PreCommit []string
		PrePush   []string
	}
}

// PrintError prints an error message
func PrintError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s%s %s%s\n", Red, IconError, msg, Reset)
}

// requireVcpkgProject ensures the current directory has a vcpkg.json manifest.
func requireVcpkgProject(cmdName string) error {
	if _, err := os.Stat("vcpkg.json"); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s requires a vcpkg project (vcpkg.json not found)\n  hint: run inside a vcpkg manifest project or create one with cpx new", cmdName)
		}
		return fmt.Errorf("failed to check vcpkg manifest: %w", err)
	}
	return nil
}

// Spinner represents a simple progress spinner
type Spinner struct {
	frames  []string
	current int
	message string
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
