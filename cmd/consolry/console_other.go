//go:build !windows

package main

// useParentConsole does nothing off Windows, where programs always print to their terminal.
func useParentConsole() {}
