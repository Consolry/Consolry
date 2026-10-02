//go:build windows

package daemon

import (
	"os/exec"
	"strconv"
)

func prepare(cmd *exec.Cmd) {}

// killTree ends the server and anything it launched, such as a Java process started by a script.
func killTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
