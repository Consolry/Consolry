package main

import (
	"os"
	"syscall"
)

// useParentConsole lets the desktop build print to the terminal it was started from, for
// options such as -reset-password. Started from an icon there is no terminal, and nothing changes.
func useParentConsole() {
	const attachParentProcess = ^uintptr(0)
	attach := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if ok, _, _ := attach.Call(attachParentProcess); ok == 0 {
		return
	}
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = out, out
	}
}
