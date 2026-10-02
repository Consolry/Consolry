// Command consolry-daemon runs on each machine that hosts game servers.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/DinoNaedYT/Consolry/internal/daemon"
	"github.com/DinoNaedYT/Consolry/internal/version"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8750", "address to listen on")
	dataDir := flag.String("data", "daemon-data", "directory for server files and daemon state")
	flag.Parse()

	manager, err := daemon.NewManager(*dataDir)
	if err != nil {
		log.Fatalf("could not open data directory: %v", err)
	}
	token, created, err := loadToken(*dataDir)
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

// loadToken returns the shared secret the panel uses to talk to this daemon.
// CONSOLRY_DAEMON_TOKEN wins; otherwise one is generated once and kept in the data directory.
func loadToken(dir string) (token string, created bool, err error) {
	if env := strings.TrimSpace(os.Getenv("CONSOLRY_DAEMON_TOKEN")); env != "" {
		return env, false, nil
	}
	path := filepath.Join(dir, "token")
	data, err := os.ReadFile(path)
	if err == nil {
		return strings.TrimSpace(string(data)), false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", false, err
	}
	token = hex.EncodeToString(raw)
	return token, true, os.WriteFile(path, []byte(token+"\n"), 0o600)
}
