package panel

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/DinoNaedYT/Consolry/internal/minecraft"
)

// splitJavaArgs separates a Java command's arguments into the memory flags, the user's
// own options, and the part from "-jar" onwards that names what to run.
func splitJavaArgs(args []string) (options, tail []string) {
	options, tail = []string{}, []string{}
	for i, arg := range args {
		if arg == "-jar" {
			return options, args[i:]
		}
		if strings.HasPrefix(arg, "-Xms") || strings.HasPrefix(arg, "-Xmx") {
			continue
		}
		options = append(options, arg)
	}
	return options, tail
}

// memoryOf reads the -Xmx value from Java arguments, in megabytes.
func memoryOf(args []string) int {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-Xmx") || len(arg) < 6 {
			continue
		}
		value, _ := strconv.Atoi(arg[4 : len(arg)-1])
		switch arg[len(arg)-1] {
		case 'G', 'g':
			return value * 1024
		case 'M', 'm':
			return value
		}
	}
	return 0
}

type startupView struct {
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	StopCommand string   `json:"stopCommand"`
	MemoryMB    int      `json:"memoryMb"`
	JavaOptions []string `json:"javaOptions"`
	// JavaNeeded is the oldest Java this server can run on, or 0 if it does not use Java.
	JavaNeeded  int      `json:"javaNeeded"`
	SystemJava  string   `json:"systemJava"`
	ManagedJava []int    `json:"managedJava"`
	Recommended []string `json:"recommended"`
}

func (a *App) handleStartup(w http.ResponseWriter, r *http.Request) {
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	spec, err := serverState(r.Context(), node, row.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	view := startupView{
		Command: spec.Command, Args: spec.Args, StopCommand: spec.StopCommand,
		MemoryMB: memoryOf(spec.Args), JavaNeeded: spec.Java, JavaOptions: []string{}, ManagedJava: []int{}, Recommended: []string{},
	}
	if view.Args == nil {
		view.Args = []string{}
	}
	if spec.Command == "java" {
		view.JavaOptions, _ = splitJavaArgs(spec.Args)
	}
	var java struct {
		Found   bool   `json:"found"`
		Version string `json:"version"`
		Managed []int  `json:"managed"`
	}
	if node.call(r.Context(), http.MethodGet, "/java", nil, &java) == nil {
		if java.Found {
			view.SystemJava = java.Version
		}
		if java.Managed != nil {
			view.ManagedJava = java.Managed
		}
	}
	if row.Kind == "minecraft" && row.Software != "fabric" {
		view.Recommended = minecraft.RecommendedFlags(r.Context(), row.MCVersion)
	}
	writeJSON(w, http.StatusOK, view)
}

// handleNetwork reports how players reach a server: its port, this machine's addresses, and whether it is listening.
func (a *App) handleNetwork(w http.ResponseWriter, r *http.Request) {
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	port := 0
	if row.Kind == "minecraft" {
		port = 25565
		if value, err := strconv.Atoi(property(readServerFile(r.Context(), node, row.ID, "server.properties"), "server-port")); err == nil && value > 0 {
			port = value
		}
	}
	var network struct {
		Addresses []string `json:"addresses"`
		Listening bool     `json:"listening"`
	}
	if err := node.call(r.Context(), http.MethodGet, "/network?port="+strconv.Itoa(port), nil, &network); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if network.Addresses == nil {
		network.Addresses = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"port": port, "addresses": network.Addresses, "listening": network.Listening})
}
