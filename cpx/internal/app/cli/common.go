package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ozacod/cpx/pkg/config"
)

// Variables for mocking in tests
var (
	execCommand  = exec.Command
	execLookPath = exec.LookPath
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
)

// Icon constants for consistent output
const (
	IconSuccess = "✓"
	IconError   = "✗"
)

// Version is the cpx version
const Version = "1.0.16"

// DefaultServer is the default server URL
const DefaultServer = "https://cpx-dev.vercel.app"

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

// ProjectType represents the type of C++ project
type ProjectType string

const (
	ProjectTypeVcpkg   ProjectType = "vcpkg"
	ProjectTypeBazel   ProjectType = "bazel"
	ProjectTypeMeson   ProjectType = "meson"
	ProjectTypeUnknown ProjectType = "unknown"
)

// DetectProjectType determines if current directory is vcpkg, bazel, meson, or unknown
func DetectProjectType() ProjectType {
	if _, err := os.Stat("vcpkg.json"); err == nil {
		return ProjectTypeVcpkg
	}
	if _, err := os.Stat("MODULE.bazel"); err == nil {
		return ProjectTypeBazel
	}
	if _, err := os.Stat("meson.build"); err == nil {
		return ProjectTypeMeson
	}
	return ProjectTypeUnknown
}

// RequireProject ensures the current directory is a cpx project (vcpkg, bazel, or meson)
func RequireProject(cmdName string) (ProjectType, error) {
	pt := DetectProjectType()
	if pt == ProjectTypeUnknown {
		return pt, fmt.Errorf("%s requires a cpx project (vcpkg.json, MODULE.bazel, or meson.build not found)\n  hint: create one with cpx new", cmdName)
	}
	return pt, nil
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

// CheckCommandExists checks if a command is available in PATH
func CheckCommandExists(command string) bool {
	_, err := execLookPath(command)
	return err == nil
}

// CheckFileExists checks if a file exists
func CheckFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CheckBuildToolsForProject checks if the required build tools are available for the project type
// Returns a list of missing tools
func CheckBuildToolsForProject(projectType ProjectType) []string {
	var missing []string

	switch projectType {
	case ProjectTypeVcpkg:
		// vcpkg projects need vcpkg, cmake, make/ninja, and compilers
		// Check for vcpkg: 1) cpx config, 2) VCPKG_ROOT env, 3) PATH
		vcpkgFound := false
		// First check cpx config
		if cfg, err := config.LoadGlobal(); err == nil && cfg.VcpkgRoot != "" {
			// Verify vcpkg executable exists at config path
			vcpkgPath := filepath.Join(cfg.VcpkgRoot, "vcpkg")
			if runtime.GOOS == "windows" {
				vcpkgPath += ".exe"
			}
			if _, err := os.Stat(vcpkgPath); err == nil {
				vcpkgFound = true
			}
		}
		// Then check VCPKG_ROOT environment variable
		if !vcpkgFound && os.Getenv("VCPKG_ROOT") != "" {
			vcpkgPath := filepath.Join(os.Getenv("VCPKG_ROOT"), "vcpkg")
			if runtime.GOOS == "windows" {
				vcpkgPath += ".exe"
			}
			if _, err := os.Stat(vcpkgPath); err == nil {
				vcpkgFound = true
			}
		}
		// Finally check PATH
		if !vcpkgFound && CheckCommandExists("vcpkg") {
			vcpkgFound = true
		}
		if !vcpkgFound {
			missing = append(missing, "vcpkg (run 'cpx config set-vcpkg-root <path>' or set VCPKG_ROOT)")
		}
		if !CheckCommandExists("cmake") {
			missing = append(missing, "cmake")
		}
		if !CheckCommandExists("make") && !CheckCommandExists("ninja") {
			missing = append(missing, "make or ninja")
		}
		hasCC := CheckCommandExists("gcc") || CheckCommandExists("clang") || CheckCommandExists("cc")
		hasCXX := CheckCommandExists("g++") || CheckCommandExists("clang++") || CheckCommandExists("c++")
		if !hasCC {
			missing = append(missing, "C compiler (gcc, clang, or cc)")
		}
		if !hasCXX {
			missing = append(missing, "C++ compiler (g++, clang++, or c++)")
		}
	case ProjectTypeBazel:
		if !CheckCommandExists("bazel") && !CheckCommandExists("bazelisk") {
			missing = append(missing, "bazel or bazelisk")
		}
	case ProjectTypeMeson:
		if !CheckCommandExists("meson") {
			missing = append(missing, "meson")
		}
		if !CheckCommandExists("ninja") {
			missing = append(missing, "ninja")
		}
		hasCC := CheckCommandExists("gcc") || CheckCommandExists("clang") || CheckCommandExists("cc")
		hasCXX := CheckCommandExists("g++") || CheckCommandExists("clang++") || CheckCommandExists("c++")
		if !hasCC {
			missing = append(missing, "C compiler (gcc, clang, or cc)")
		}
		if !hasCXX {
			missing = append(missing, "C++ compiler (g++, clang++, or c++)")
		}
	case ProjectTypeUnknown:
		// For unknown projects, assume CMake-based
		if !CheckCommandExists("cmake") {
			missing = append(missing, "cmake")
		}
		if !CheckCommandExists("make") && !CheckCommandExists("ninja") {
			missing = append(missing, "make or ninja")
		}
		hasCC := CheckCommandExists("gcc") || CheckCommandExists("clang") || CheckCommandExists("cc")
		hasCXX := CheckCommandExists("g++") || CheckCommandExists("clang++") || CheckCommandExists("c++")
		if !hasCC {
			missing = append(missing, "C compiler (gcc, clang, or cc)")
		}
		if !hasCXX {
			missing = append(missing, "C++ compiler (g++, clang++, or c++)")
		}
	}

	return missing
}

// WarnMissingBuildTools checks for missing build tools and prints a warning
// Returns the list of missing tools (empty if all tools are present)
func WarnMissingBuildTools(projectType ProjectType) []string {
	missing := CheckBuildToolsForProject(projectType)
	if len(missing) > 0 {
		fmt.Printf("%s%s Warning: Some build tools are missing:%s\n", Yellow, IconError, Reset)
		for _, tool := range missing {
			fmt.Printf("  - %s\n", tool)
		}
		fmt.Println()
	}
	return missing
}
