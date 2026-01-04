package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ozacod/cpx/internal/config"
	"github.com/ozacod/cpx/internal/utils"
	"github.com/ozacod/cpx/internal/utils/colors"
	"github.com/spf13/cobra"
)

func EnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print environment information",
		Long:  "Print information about the current environment including OS, compilers, and build tools.",
		RunE:  runEnv,
	}

	return cmd
}

func runEnv(_ *cobra.Command, _ []string) error {
	fmt.Printf("%s=== cpx Environment ===%s\n", colors.Bold, colors.Reset)
	fmt.Println()

	fmt.Printf("%sCPX%s\n", colors.Bold, colors.Reset)
	fmt.Printf("  Version:     %s\n", utils.Version)
	fmt.Printf("  GOOS:        %s\n", runtime.GOOS)
	fmt.Printf("  GOARCH:      %s\n", runtime.GOARCH)
	fmt.Printf("  CPX root:    %s\n", getCpxRoot())
	fmt.Println()

	fmt.Printf("%sOperating System%s\n", colors.Bold, colors.Reset)
	fmt.Printf("  OS:          %s %s\n", runtime.GOOS, getOSVersion())
	fmt.Printf("  CPUs:        %d\n", runtime.NumCPU())
	fmt.Printf("  GO version:  %s\n", runtime.Version())
	fmt.Println()

	fmt.Printf("%sCompilers%s\n", colors.Bold, colors.Reset)
	showCompiler("CC", "C compiler")
	showCompiler("CXX", "C++ compiler")
	fmt.Println()

	fmt.Printf("%sBuild Tools%s\n", colors.Bold, colors.Reset)
	showTool("cmake", "CMake")
	showTool("make", "Make")
	showTool("ninja", "Ninja")
	showTool("meson", "Meson")
	showTool("bazel", "Bazel")
	showVcpkg()
	fmt.Println()

	fmt.Printf("%sEnvironment Variables%s\n", colors.Bold, colors.Reset)
	showEnvVar("VCPKG_ROOT")
	showEnvVar("CC")
	showEnvVar("CXX")
	showEnvVar("CMAKE_PREFIX_PATH")
	showEnvVar("CPX_TOOLCHAIN")
	fmt.Println()

	return nil
}

func getCpxRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "<unknown>"
	}
	if runtime.GOOS == "windows" {
		return home + "\\.config\\cpx"
	}
	return home + "/.config/cpx"
}

func getOSVersion() string {
	if runtime.GOOS == "darwin" {
		if version := runCommand("sw_vers", "-productVersion"); version != "" {
			return version
		}
	}
	if runtime.GOOS == "linux" {
		if version := runCommand("cat", "/etc/os-release"); version != "" {
			for _, line := range strings.Split(version, "\n") {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					return strings.TrimPrefix(line, "PRETTY_NAME=")
				}
			}
		}
	}
	return ""
}

func showCompiler(envVar, description string) {
	compiler := os.Getenv(envVar)
	if compiler == "" {
		compiler = detectCompiler(envVar)
	}

	path := runCommand("which", compiler)
	version := runCommand(compiler, "--version")

	fmt.Printf("  %-12s %s\n", description+":", compiler)
	if path != "" {
		fmt.Printf("    Path:    %s\n", path)
	}
	if version != "" {
		lines := strings.Split(version, "\n")
		if len(lines) > 0 {
			versionLine := strings.TrimSpace(lines[0])
			if len(versionLine) > 60 {
				versionLine = versionLine[:60] + "..."
			}
			fmt.Printf("    Version: %s\n", versionLine)
		}
	}
}

func showTool(name, description string) {
	path := runCommand("which", name)
	if path == "" {
		fmt.Printf("  %-12s %snot found%s\n", description+":", colors.Yellow, colors.Reset)
		return
	}

	fmt.Printf("  %-12s %s\n", description+":", path)
}

func showVcpkg() {
	// Check cpx config first
	if cfg, err := config.LoadGlobal(); err == nil && cfg.VcpkgRoot != "" {
		vcpkgPath := filepath.Join(cfg.VcpkgRoot, "vcpkg")
		if runtime.GOOS == "windows" {
			vcpkgPath += ".exe"
		}
		if _, err := os.Stat(vcpkgPath); err == nil {
			fmt.Printf("  %-12s %s %s(cpx bundled)%s\n", "vcpkg:", vcpkgPath, colors.Green, colors.Reset)
			return
		}
	}

	// Check VCPKG_ROOT environment variable
	if vcpkgRoot := os.Getenv("VCPKG_ROOT"); vcpkgRoot != "" {
		vcpkgPath := filepath.Join(vcpkgRoot, "vcpkg")
		if runtime.GOOS == "windows" {
			vcpkgPath += ".exe"
		}
		if _, err := os.Stat(vcpkgPath); err == nil {
			fmt.Printf("  %-12s %s %s(VCPKG_ROOT)%s\n", "vcpkg:", vcpkgPath, colors.Cyan, colors.Reset)
			return
		}
	}

	// Check PATH
	if path := runCommand("which", "vcpkg"); path != "" {
		fmt.Printf("  %-12s %s\n", "vcpkg:", path)
		return
	}

	fmt.Printf("  %-12s %snot found%s\n", "vcpkg:", colors.Yellow, colors.Reset)
}

func showEnvVar(name string) {
	value := os.Getenv(name)
	if value == "" {
		fmt.Printf("  %-18s (not set)\n", name)
	} else {
		display := value
		if len(display) > 50 {
			display = display[:50] + "..."
		}
		fmt.Printf("  %-18s %s\n", name, display)
	}
}

func detectCompiler(envVar string) string {
	switch envVar {
	case "CC":
		compilers := []string{"gcc", "clang", "cc"}
		for _, c := range compilers {
			if path := runCommand("which", c); path != "" {
				return c
			}
		}
	case "CXX":
		compilers := []string{"g++", "clang++", "c++"}
		for _, c := range compilers {
			if path := runCommand("which", c); path != "" {
				return c
			}
		}
	}
	return ""
}

func runCommand(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
