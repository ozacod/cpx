package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ozacod/cpx/internal/pkg/build"
	"github.com/spf13/cobra"
)

var checkSetupVcpkgEnvFunc func() error

// CheckCmd creates the check command
func CheckCmd(setupVcpkgEnv func() error) *cobra.Command {
	checkSetupVcpkgEnvFunc = setupVcpkgEnv

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check code compiles with sanitizers",
		Long:  "Check code compiles with sanitizers (--asan, --tsan, --msan, --ubsan). Only one sanitizer can be enabled at a time.",
		RunE:  runCheck,
	}

	cmd.Flags().Bool("asan", false, "Build with AddressSanitizer")
	cmd.Flags().Bool("tsan", false, "Build with ThreadSanitizer")
	cmd.Flags().Bool("msan", false, "Build with MemorySanitizer")
	cmd.Flags().Bool("ubsan", false, "Build with UndefinedBehaviorSanitizer")

	return cmd
}

func runCheck(cmd *cobra.Command, _ []string) error {
	asan, _ := cmd.Flags().GetBool("asan")
	tsan, _ := cmd.Flags().GetBool("tsan")
	msan, _ := cmd.Flags().GetBool("msan")
	ubsan, _ := cmd.Flags().GetBool("ubsan")

	// Determine which sanitizer to use
	var sanitizer string
	sanitizerCount := 0
	if asan {
		sanitizer = "address"
		sanitizerCount++
	}
	if tsan {
		if sanitizerCount > 0 {
			return fmt.Errorf("only one sanitizer can be enabled at a time")
		}
		sanitizer = "thread"
		sanitizerCount++
	}
	if msan {
		if sanitizerCount > 0 {
			return fmt.Errorf("only one sanitizer can be enabled at a time")
		}
		sanitizer = "memory"
		sanitizerCount++
	}
	if ubsan {
		if sanitizerCount > 0 {
			return fmt.Errorf("only one sanitizer can be enabled at a time")
		}
		sanitizer = "undefined"
		sanitizerCount++
	}

	if sanitizer == "" {
		return fmt.Errorf("no sanitizer specified. Use --asan, --tsan, --msan, or --ubsan")
	}

	return checkCode(sanitizer, checkSetupVcpkgEnvFunc)
}

func checkCode(sanitizer string, setupVcpkgEnv func() error) error {
	// Set VCPKG_ROOT from cpx config if not already set
	if err := setupVcpkgEnv(); err != nil {
		return err
	}

	projectName := build.GetProjectNameFromCMakeLists()
	if projectName == "" {
		projectName = "project"
	}

	fmt.Printf("%s Running sanitizer check (%s)...%s\n", Cyan, sanitizer, Reset)

	buildDir := "build"
	os.RemoveAll(buildDir) // Clean build for sanitizer

	// Configure CMake with sanitizer flags
	buildType := "Debug" // Sanitizers work best with Debug builds
	var cxxFlags, linkerFlags string

	switch sanitizer {
	case "address":
		cxxFlags = "-fsanitize=address -fno-omit-frame-pointer"
		linkerFlags = "-fsanitize=address"
	case "thread":
		cxxFlags = "-fsanitize=thread"
		linkerFlags = "-fsanitize=thread"
	case "memory":
		cxxFlags = "-fsanitize=memory -fno-omit-frame-pointer"
		linkerFlags = "-fsanitize=memory"
	case "undefined":
		cxxFlags = "-fsanitize=undefined -fno-omit-frame-pointer"
		linkerFlags = "-fsanitize=undefined"
	default:
		return fmt.Errorf("unknown sanitizer: %s", sanitizer)
	}

	cmakeArgs := []string{
		"-B", buildDir,
		"-DCMAKE_BUILD_TYPE=" + buildType,
		"-DCMAKE_CXX_FLAGS=" + cxxFlags,
		"-DCMAKE_EXE_LINKER_FLAGS=" + linkerFlags,
		"-DCMAKE_SHARED_LINKER_FLAGS=" + linkerFlags,
	}

	// Check if CMakePresets.json exists, use preset if available
	if _, err := os.Stat("CMakePresets.json"); err == nil {
		// For presets, we need to set flags via environment or modify preset
		// For now, use traditional configure
		cmd := exec.Command("cmake", cmakeArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cmake configure failed: %w", err)
		}
	} else {
		cmd := exec.Command("cmake", cmakeArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cmake configure failed: %w", err)
		}
	}

	// Build
	fmt.Printf("%s Building with sanitizer...%s\n", Cyan, Reset)
	buildCmd := exec.Command("cmake", "--build", buildDir, "--config", buildType)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Printf("%s Build with %s sanitizer complete!%s\n", Green, sanitizer, Reset)
	fmt.Printf("%s Run the executable to detect issues%s\n", Yellow, Reset)

	return nil
}
