// Command consolry is the web panel.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/DinoNaedYT/Consolry/internal/panel"
	"github.com/DinoNaedYT/Consolry/internal/version"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8700", "address to listen on")
	dbPath := flag.String("db", "consolry.db", "path to the SQLite database file")
	flag.Parse()

	store, err := panel.OpenStore(*dbPath)
	if err != nil {
		log.Fatalf("could not open database: %v", err)
	}
	defer store.Close()

	server := &http.Server{
		Addr:              *listen,
		Handler:           panel.New(store, version.Version).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	log.Printf("consolry %s panel listening on http://%s", version.Version, *listen)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
