package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ozacod/cpx/internal/build/common"
	"github.com/ozacod/cpx/internal/config"
	"github.com/ozacod/cpx/internal/utils/colors"
)

var (
	ExecLookPath = exec.LookPath
)

func CheckCommandExists(command string) bool {
	_, err := ExecLookPath(command)
	return err == nil
}

func CheckFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

const Version = "1.3.5"

const DefaultServer = "https://cpx-dev.vercel.app"

func PrintError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s✗ %s%s\n", colors.Red, msg, colors.Reset)
}

func PrintSuccess(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s✓ %s%s\n", colors.Green, msg, colors.Reset)
}

func PrintWarning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s! %s%s\n", colors.Yellow, msg, colors.Reset)
}

func PrintInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s• %s%s\n", colors.Cyan, msg, colors.Reset)
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
	if _, err := os.Stat(common.VcpkgManifest); err == nil {
		return ProjectTypeVcpkg
	}
	if _, err := os.Stat(common.BazelModuleFile); err == nil {
		return ProjectTypeBazel
	}
	if _, err := os.Stat(common.MesonBuildFile); err == nil {
		return ProjectTypeMeson
	}
	// CMake-only: has CMakeLists.txt but no vcpkg.json
	if _, err := os.Stat(common.CMakeListsFile); err == nil {
		return ProjectTypeCMake
	}
	return ProjectTypeUnknown
}

func RequireProject(cmdName string) (ProjectType, error) {
	pt := DetectProjectType()
	if pt == ProjectTypeUnknown {
		return pt, fmt.Errorf("%s requires a cpx project (%s, %s, %s, or %s not found)\n  hint: create one with cpx new",
			cmdName, common.VcpkgManifest, common.BazelModuleFile, common.MesonBuildFile, common.CMakeListsFile)
	}
	return pt, nil
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
