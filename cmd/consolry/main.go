// Command consolry is the Consolry panel. It includes a daemon, so on a single machine
// this one program is all that needs to run.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/DinoNaedYT/Consolry/internal/daemon"
	"github.com/DinoNaedYT/Consolry/internal/panel"
	"github.com/DinoNaedYT/Consolry/internal/version"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8700", "address the panel listens on")
	dbPath := flag.String("db", "consolry.db", "path to the SQLite database file")
	dataDir := flag.String("data", "daemon-data", "directory for server files, backups and Java")
	localListen := flag.String("local-daemon", "127.0.0.1:8750", "address the built-in daemon listens on")
	noLocal := flag.Bool("no-local-daemon", false, "do not run servers on this machine; use only nodes added by hand")
	flag.Parse()

	store, err := panel.OpenStore(*dbPath)
	if err != nil {
		log.Fatalf("could not open database: %v", err)
	}
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if !*noLocal {
		shutdown := startLocalDaemon(store, *localListen, *dataDir)
		defer shutdown()
	}

	app := panel.New(store, version.Version)
	app.StartScheduler(ctx)
	server := &http.Server{
		Addr:              *listen,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	log.Printf("consolry %s is ready: open http://%s in your browser", version.Version, *listen)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Print(err)
	}
}

// startLocalDaemon runs the daemon inside this program and registers it as the node
// "This machine". It returns a function that stops every running server on exit.
func startLocalDaemon(store *panel.Store, address, dataDir string) func() {
	token, _, err := daemon.LoadToken(dataDir)
	if err != nil {
		log.Fatalf("could not prepare the data directory: %v", err)
	}
	if err := store.EnsureLocalNode("http://"+address, token); err != nil {
		log.Fatalf("could not register this machine as a node: %v", err)
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		// Most likely a standalone consolry-daemon is already running here; use that one.
		log.Printf("built-in daemon not started (%v); using the daemon already on %s", err, address)
		return func() {}
	}
	manager, err := daemon.NewManager(dataDir)
	if err != nil {
		log.Fatalf("could not open the data directory: %v", err)
	}
	server := &http.Server{Handler: daemon.Handler(manager, token, version.Version), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("built-in daemon stopped: %v", err)
		}
	}()
	return func() {
		log.Print("stopping servers")
		manager.Shutdown()
		_ = server.Close()
	}
}
