// Package testutil provides shared test utilities for build system tests.
package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestHelperProcess isn't a real test. It's used as a helper process
// for mocking exec.Command. Call this at the start of any test file that
// uses MockExecCommand.
//
// Usage in test files:
//
//	func TestHelperProcess(t *testing.T) {
//		testutil.TestHelperProcess(t)
//	}
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if out := os.Getenv("MOCK_OUTPUT"); out != "" {
		fmt.Print(out)
	}
	os.Exit(0)
}

// MockExecCommand returns a function that can be used to mock exec.Command.
// It captures all command invocations in the provided capturedArgs slice.
// The mock command runs the test binary with -test.run=TestHelperProcess
// which exits successfully without actually running the command.
func MockExecCommand(capturedArgs *[][]string) func(string, ...string) *exec.Cmd {
	return func(name string, arg ...string) *exec.Cmd {
		args := append([]string{name}, arg...)
		*capturedArgs = append(*capturedArgs, args)

		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
}

// MockExecCommandWithOutput is like MockExecCommand but allows setting custom
// output for specific commands via the outputFunc callback.
func MockExecCommandWithOutput(capturedArgs *[][]string, outputFunc func(name string, args []string) string) func(string, ...string) *exec.Cmd {
	return func(name string, arg ...string) *exec.Cmd {
		args := append([]string{name}, arg...)
		*capturedArgs = append(*capturedArgs, args)

		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		env := append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

		if outputFunc != nil {
			if output := outputFunc(name, arg); output != "" {
				env = append(env, "MOCK_OUTPUT="+output)
			}
		}

		cmd.Env = env
		return cmd
	}
}
