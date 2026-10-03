// Command consolry is the Consolry panel. It includes a daemon, so on a single machine
// this one program is all that needs to run.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/DinoNaedYT/Consolry/internal/daemon"
	"github.com/DinoNaedYT/Consolry/internal/panel"
	"github.com/DinoNaedYT/Consolry/internal/version"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8700", "address the panel listens on")
	dir := flag.String("dir", "", "folder for all of Consolry's data (default: a Consolry folder in your user profile)")
	localListen := flag.String("local-daemon", "127.0.0.1:8750", "address the built-in daemon listens on")
	noLocal := flag.Bool("no-local-daemon", false, "do not run servers on this machine; use only nodes added by hand")
	noBrowser := flag.Bool("no-browser", false, "do not open the panel in a browser on start")
	autostart := flag.String("autostart", "", `"on" or "off": start Consolry automatically when you sign in, then exit`)
	flag.Parse()

	home, err := dataFolder(*dir)
	if err != nil {
		fatal("could not prepare the data folder: %v", err)
	}

	if *autostart != "" {
		if err := setAutostart(*autostart == "on", home); err != nil {
			fatal("could not change the start-up setting: %v", err)
		}
		fmt.Println("Start when you sign in:", *autostart)
		return
	}

	// Keep a log on disk as well: when started from an icon there is no window to read it in.
	if file, err := os.OpenFile(filepath.Join(home, "consolry.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		defer file.Close()
		log.SetOutput(io.MultiWriter(file, os.Stderr))
	}
	log.Printf("consolry %s, data in %s", version.Version, home)

	// An update leaves the previous program beside the new one until the new one has started.
	if program, err := os.Executable(); err == nil {
		_ = os.Remove(program + ".old")
	}

	address := "http://" + *listen

	// Listen before anything else, so a second copy notices at once and just shows the first.
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Printf("not starting: %v. Consolry is probably already running.", err)
		if !*noBrowser {
			openBrowser(address)
		}
		return
	}

	store, err := panel.OpenStore(filepath.Join(home, "consolry.db"))
	if err != nil {
		fatal("could not open database: %v", err)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	stopServers := func(bool) {}
	if !*noLocal {
		stopServers = startLocalDaemon(store, *localListen, filepath.Join(home, "daemon-data"))
	}

	app := panel.New(store, version.Version)
	// After an update Consolry closes as usual, then starts the new program in its place.
	var restarting atomic.Bool
	app.OnRestart(func() {
		restarting.Store(true)
		stop()
	})
	app.StartScheduler(ctx)
	app.StartWatching(ctx)
	handler := app.Handler()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	if host, port, err := net.SplitHostPort(*listen); err == nil {
		if number, err := strconv.Atoi(port); err == nil {
			app.StartRemote(ctx, handler, host, number)
		}
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Print(err)
			stop()
		}
	}()

	log.Printf("ready: open %s in your browser", address)
	if !*noBrowser {
		openBrowser(address)
	}

	// Blocks until Consolry is asked to quit: from the tray icon, Ctrl+C, or the system shutting down.
	waitForExit(ctx, stop, address, home)

	log.Print("stopping: saving and closing servers")
	stopServers(restarting.Load())
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	log.Print("stopped")
	if restarting.Load() {
		store.Close()
		startAgain()
	}
}

// startAgain runs the program that an update has just put in place, with the same options.
func startAgain() {
	// A background service is started again by the system when it exits with an error.
	if os.Getenv("INVOCATION_ID") != "" {
		log.Print("updated: leaving it to the service to start the new version")
		os.Exit(1)
	}
	program, err := os.Executable()
	if err != nil {
		fatal("updated, but could not start the new version: %v", err)
	}
	args := os.Args[1:]
	quiet := false
	for _, arg := range args {
		quiet = quiet || arg == "-no-browser" || arg == "--no-browser"
	}
	if !quiet {
		args = append(args, "-no-browser")
	}
	command := exec.Command(program, args...)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Start(); err != nil {
		fatal("updated, but could not start the new version: %v", err)
	}
	log.Print("updated: the new version is starting")
	os.Exit(0)
}

func fatal(format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(1)
}

// dataFolder decides where Consolry keeps its database, servers and backups.
// A folder that already holds data wins, so an existing install keeps working wherever it was started.
func dataFolder(chosen string) (string, error) {
	if chosen == "" {
		if _, err := os.Stat("consolry.db"); err == nil {
			chosen = "."
		} else if runtime.GOOS == "windows" && os.Getenv("LOCALAPPDATA") != "" {
			chosen = filepath.Join(os.Getenv("LOCALAPPDATA"), "Consolry")
		} else if data := os.Getenv("XDG_DATA_HOME"); data != "" {
			chosen = filepath.Join(data, "consolry")
		} else if user, err := os.UserHomeDir(); err == nil {
			chosen = filepath.Join(user, ".local", "share", "consolry")
		} else {
			chosen = "."
		}
	}
	absolute, err := filepath.Abs(chosen)
	if err != nil {
		return "", err
	}
	return absolute, os.MkdirAll(absolute, 0o755)
}

// openBrowser shows the panel on a desktop machine. On a server with no desktop it quietly does nothing.
func openBrowser(address string) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)
	case "darwin":
		command = exec.Command("open", address)
	default:
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return
		}
		command = exec.Command("xdg-open", address)
	}
	_ = command.Start()
}

// startLocalDaemon runs the daemon inside this program and registers it as the node
// "This machine". It returns a function that stops every running server cleanly; asked to
// remember, it also notes which were running so the next start brings them back.
func startLocalDaemon(store *panel.Store, address, dataDir string) func(remember bool) {
	token, _, err := daemon.LoadToken(dataDir)
	if err != nil {
		fatal("could not prepare the data directory: %v", err)
	}
	if err := store.EnsureLocalNode("http://"+address, token); err != nil {
		fatal("could not register this machine as a node: %v", err)
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		// Most likely a standalone consolry-daemon is already running here; use that one.
		log.Printf("built-in daemon not started (%v); using the daemon already on %s", err, address)
		return func(bool) {}
	}
	manager, err := daemon.NewManager(dataDir)
	if err != nil {
		fatal("could not open the data directory: %v", err)
	}

	// Servers that were running when Consolry restarted itself for an update come back up.
	resume := filepath.Join(dataDir, "resume.json")
	if data, err := os.ReadFile(resume); err == nil {
		_ = os.Remove(resume)
		var ids []string
		_ = json.Unmarshal(data, &ids)
		for _, id := range ids {
			if s, err := manager.Get(id); err == nil {
				if err := s.Start(); err != nil {
					log.Printf("could not start %s again after the update: %v", id, err)
				}
			}
		}
	}
	server := &http.Server{Handler: daemon.Handler(manager, token, version.Version), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("built-in daemon stopped: %v", err)
		}
	}()
	return func(remember bool) {
		if remember {
			ids := []string{}
			for _, info := range manager.List() {
				if info.State == daemon.StateRunning || info.State == daemon.StateStarting {
					ids = append(ids, info.ID)
				}
			}
			if data, err := json.Marshal(ids); err == nil {
				_ = os.WriteFile(resume, data, 0o600)
			}
		}
		manager.Shutdown()
		_ = server.Close()
	}
}
