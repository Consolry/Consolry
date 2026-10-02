// Package testhelper lets a test binary stand in for a game server.
package testhelper

import (
	"bufio"
	"fmt"
	"os"
)

// EchoArg, when passed to a test binary, makes it behave like a tiny game server.
const EchoArg = "consolry-echo-server"

// RunEchoIfRequested turns the process into the echo server and exits if EchoArg is present.
// Call it first in TestMain. The echo server repeats each line it is sent and quits on "stop".
func RunEchoIfRequested() {
	if len(os.Args) < 2 || os.Args[1] != EchoArg {
		return
	}
	fmt.Println("ready")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "stop" {
			fmt.Println("bye")
			os.Exit(0)
		}
		if line == "crash" {
			os.Exit(3)
		}
		fmt.Println("echo: " + line)
	}
	os.Exit(0)
}
