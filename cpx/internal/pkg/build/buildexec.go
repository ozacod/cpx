package build

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/schollz/progressbar/v3"
)

var progressRe = regexp.MustCompile(`^\[\s*\d+%]`)

// runCMakeBuild runs "cmake --build" with optional verbose output.
// If verbose is false, it streams only progress lines like "[ 93%]" and errors.
func runCMakeBuild(buildArgs []string, verbose bool, currentStep, totalSteps int) error {
	cmd := exec.Command("cmake", buildArgs...)

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

// runCMakeConfigure runs cmake configure quietly unless verbose is true.
func runCMakeConfigure(cmd *exec.Cmd, verbose bool) error {
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
