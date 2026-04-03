package utils

import (
	"os/exec"
	"runtime"
)

func ProcessExists(name string) bool {
	var out []byte
	var err error

	if runtime.GOOS == "darwin" {
		out, err = exec.Command("pgrep", "-x", name).Output()
	} else {
		out, err = exec.Command("pidof", name).Output()
	}

	return err == nil && len(out) > 0
}

func ProcessExistsWithExit(name string) {
	if ProcessExists(name) {
		println("process", name, "already running")
		exec.Command("pkill", "-f", name).Run()
		println("killed existing process", name)
	}
}
