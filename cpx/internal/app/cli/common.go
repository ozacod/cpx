package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ozacod/cpx/internal/pkg/utils/colors"
	"github.com/ozacod/cpx/internal/pkg/utils/common"
	"github.com/ozacod/cpx/pkg/config"
)

var (
	execLookPath = exec.LookPath
)

func CheckCommandExists(command string) bool {
	return common.CheckCommandExists(command)
}

func CheckFileExists(path string) bool {
	return common.CheckFileExists(path)
}

const Version = "1.3.1"

const DefaultServer = "https://cpx-dev.vercel.app"

func PrintError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s✗ %s%s\n", colors.Red, msg, colors.Reset)
}

func requireVcpkgProject(cmdName string) error {
	if _, err := os.Stat("vcpkg.json"); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s requires a vcpkg project (vcpkg.json not found)\n  hint: run inside a vcpkg manifest project or create one with cpx new", cmdName)
		}
		return fmt.Errorf("failed to check vcpkg manifest: %w", err)
	}
	return nil
}

type ProjectType string

const (
	ProjectTypeVcpkg   ProjectType = "vcpkg"
	ProjectTypeBazel   ProjectType = "bazel"
	ProjectTypeMeson   ProjectType = "meson"
	ProjectTypeCMake   ProjectType = "cmake"
	ProjectTypeUnknown ProjectType = "unknown"
)

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
	// CMake-only: has CMakeLists.txt but no vcpkg.json
	if _, err := os.Stat("CMakeLists.txt"); err == nil {
		return ProjectTypeCMake
	}
	return ProjectTypeUnknown
}

func RequireProject(cmdName string) (ProjectType, error) {
	pt := DetectProjectType()
	if pt == ProjectTypeUnknown {
		return pt, fmt.Errorf("%s requires a cpx project (vcpkg.json, MODULE.bazel, meson.build, or CMakeLists.txt not found)\n  hint: create one with cpx new", cmdName)
	}
	return pt, nil
}

type Spinner struct {
	frames  []string
	current int
	message string
}

func (s *Spinner) Tick() {
	fmt.Printf("\r%s%s%s %s", colors.Cyan, s.frames[s.current], colors.Reset, s.message)
	s.current = (s.current + 1) % len(s.frames)
}

func (s *Spinner) Done(message string) {
	fmt.Printf("\r%s✓ %s%s\n", colors.Green, message, colors.Reset)
}
func (s *Spinner) Fail(message string) {
	fmt.Printf("\r%s✗ %s%s\n", colors.Red, message, colors.Reset)
}

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
	case ProjectTypeCMake, ProjectTypeUnknown:
		// CMake-only projects need cmake, make/ninja, and compilers
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
		fmt.Printf("%s✗ Warning: Some build tools are missing:%s\n", colors.Yellow, colors.Reset)
		for _, tool := range missing {
			fmt.Printf("  - %s\n", tool)
		}
		fmt.Println()
	}
	return missing
}
