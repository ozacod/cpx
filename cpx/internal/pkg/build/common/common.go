// Package common provides shared utilities for CMake-based build systems.
package common

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"

	"github.com/schollz/progressbar/v3"

	"github.com/ozacod/cpx/internal/pkg/utils/colors"
)

// ExecCommand is a variable that points to exec.Command by default.
// It can be overridden in tests to mock command execution.
var ExecCommand = exec.Command

func GetProjectNameFromCMakeLists() string {
	cmakeListsPath := "CMakeLists.txt"
	data, err := os.ReadFile(cmakeListsPath)
	if err != nil {
		return ""
	}

	// Look for: project(PROJECT_NAME ...)
	re := regexp.MustCompile(`project\s*\(\s*([^\s\)]+)`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func RemoveDir(path string) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("%s  Removing %s...%s\n", colors.Cyan, path, colors.Reset)
		if err := os.RemoveAll(path); err != nil {
			fmt.Printf("%s⚠ Failed to remove %s: %v%s\n", colors.Yellow, path, err, colors.Reset)
		}
	}
}

func DetermineBuildType(release bool, optLevel string) (string, string) {
	buildType := "Debug"
	cxxFlags := ""

	if release {
		buildType = "Release"
	}

	// Handle optimization level
	switch optLevel {
	case "0":
		cxxFlags = "-O0"
		buildType = "Debug"
	case "1":
		cxxFlags = "-O1"
		buildType = "RelWithDebInfo"
	case "2":
		cxxFlags = "-O2"
		buildType = "Release"
	case "3":
		cxxFlags = "-O3"
		buildType = "Release"
	case "s":
		cxxFlags = "-Os"
		buildType = "MinSizeRel"
	case "fast":
		cxxFlags = "-Ofast"
		buildType = "Release"
	}

	return buildType, cxxFlags
}

func GetSanitizerFlags(sanitizer string) (string, string) {
	cxxFlags := ""
	linkerFlags := ""
	switch sanitizer {
	case "asan":
		cxxFlags = " -fsanitize=address -fno-omit-frame-pointer"
		linkerFlags = "-fsanitize=address"
	case "tsan":
		cxxFlags = " -fsanitize=thread"
		linkerFlags = "-fsanitize=thread"
	case "msan":
		cxxFlags = " -fsanitize=memory -fno-omit-frame-pointer"
		linkerFlags = "-fsanitize=memory"
	case "ubsan":
		cxxFlags = " -fsanitize=undefined"
		linkerFlags = "-fsanitize=undefined"
	}
	return cxxFlags, linkerFlags
}

func FindExecutables(buildDir string) ([]string, error) {
	var executables []string

	entries, err := os.ReadDir(buildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read build directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()

		// Skip test executables and common non-executable files
		if strings.Contains(name, "_test") || strings.Contains(name, "_tests") ||
			strings.HasSuffix(name, ".a") || strings.HasSuffix(name, ".so") ||
			strings.HasSuffix(name, ".dylib") || strings.HasSuffix(name, ".dll") ||
			strings.HasSuffix(name, ".lib") || strings.HasSuffix(name, ".o") ||
			strings.HasSuffix(name, ".cmake") || strings.HasSuffix(name, ".ninja") ||
			strings.HasSuffix(name, ".make") || strings.HasSuffix(name, ".txt") {
			continue
		}

		// Check if it's executable
		if runtime.GOOS == "windows" {
			if strings.HasSuffix(name, ".exe") {
				executables = append(executables, filepath.Join(buildDir, name))
			}
		} else {
			if info.Mode()&0111 != 0 {
				executables = append(executables, filepath.Join(buildDir, name))
			}
		}
	}

	// Sort by name for consistent ordering
	sort.Strings(executables)

	return executables, nil
}

func CopyAndSign(src, dest string) error {
	// Remove destination to ensure clean copy
	os.Remove(dest)

	// Simple copy for Windows
	if runtime.GOOS == "windows" {
		input, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, input, 0755)
	}

	// Use cp -f on unix-like systems to preserve attributes
	cmd := exec.Command("cp", "-f", src, dest)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// On macOS/Darwin, force ad-hoc codesign
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("codesign", "-s", "-", "--force", dest)
		// We ignore error here because codesign might not be available or needed
		// , but it fixes the ASan issue most of the time
		_ = cmd.Run()
	}
	return nil
}

var progressRe = regexp.MustCompile(`^\[\s*\d+%]`)

func RunCMakeBuild(buildArgs []string, verbose bool, currentStep, totalSteps int) error {
	cmd := ExecCommand("cmake", buildArgs...)

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Create a progress bar for the build percentage
	bar := progressbar.NewOptions(100,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetWidth(20),
		progressbar.OptionSetDescription(fmt.Sprintf("[cyan][%d/%d][reset] Compiling", currentStep, totalSteps)),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[cyan]█[reset]",
			SaucerHead:    "[cyan]▸[reset]",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionClearOnFinish(),
	)

	// Ensure cursor is restored on interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		_ = bar.Clear()
		fmt.Print("\033[?25h") // Show cursor
		os.Exit(1)
	}()

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return err
	}

	var nonProgress bytes.Buffer
	lastPercent := -1

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		pw.Close()
	}()

	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for sc.Scan() {
		line := sc.Text()
		if match := progressRe.FindString(line); match != "" {
			pct := extractPercent(match)
			if pct >= 0 && pct != lastPercent {
				_ = bar.Set(pct)
				lastPercent = pct
			}
			continue
		}
		nonProgress.WriteString(line)
		nonProgress.WriteByte('\n')
	}

	err := <-waitCh

	// Complete the progress bar
	_ = bar.Set(100)
	_ = bar.Clear()

	if err != nil {
		if nonProgress.Len() > 0 {
			fmt.Fprintln(os.Stderr, nonProgress.String())
		}
		return err
	}

	return nil
}

func extractPercent(line string) int {
	// line format: [ 93%] ...
	start := strings.Index(line, "[")
	end := strings.Index(line, "%")
	if start == -1 || end == -1 || end <= start {
		return -1
	}
	var pct int
	if _, err := fmt.Sscanf(line[start+1:end], "%d", &pct); err != nil {
		return -1
	}
	return pct
}

func RunCMakeConfigure(cmd *exec.Cmd, verbose bool) error {
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\n%s", err, buf.String())
	}
	return nil
}

func GetBuildOptLabel(release bool, optLevel, sanitizer string) string {
	optLabel := "default (-O0)"
	if release {
		optLabel = "-O2 (Release)"
	}
	if optLevel != "" {
		optLabel = "-O" + optLevel
	}
	if sanitizer != "" {
		optLabel += "+" + sanitizer
	}
	return optLabel
}

func RemoveDirsMatchingPattern(pattern string, dirsOnly bool) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if dirsOnly && !entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match(pattern, entry.Name())
		if matched {
			RemoveDir(entry.Name())
		}
	}
}
