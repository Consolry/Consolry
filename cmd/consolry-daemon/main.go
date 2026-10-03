// Command consolry-daemon runs on an extra machine that hosts game servers.
// The panel has a daemon built in, so this is only needed for a second machine.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/Consolry/Consolry/internal/daemon"
	"github.com/Consolry/Consolry/internal/version"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8750", "address to listen on")
	dataDir := flag.String("data", "daemon-data", "directory for server files and daemon state")
	flag.Parse()

	manager, err := daemon.NewManager(*dataDir)
	if err != nil {
		log.Fatalf("could not open data directory: %v", err)
	}
	token, created, err := daemon.LoadToken(*dataDir)
	if err != nil {
		log.Fatalf("could not read daemon token: %v", err)
	}

	server := &http.Server{
		Addr:              *listen,
		Handler:           daemon.Handler(manager, token, version.Version),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("consolry-daemon %s listening on http://%s", version.Version, *listen)
	if created {
		log.Printf("new daemon token (enter this when adding the node in the panel): %s", token)
	} else {
		log.Printf("daemon token is stored in %s", filepath.Join(*dataDir, "token"))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Print("shutting down, stopping servers")
		manager.Shutdown()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
