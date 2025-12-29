package common

import (
	"os"
	"os/exec"
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
