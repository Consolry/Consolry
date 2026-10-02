package daemon

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"runtime"
	"strings"

	"github.com/coder/websocket"
)

// Handler serves the daemon API. Every request must carry the shared token.
func Handler(m *Manager, token, version string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": version, "os": runtime.GOOS, "arch": runtime.GOARCH})
	})

	mux.HandleFunc("GET /servers", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, m.List())
	})

	mux.HandleFunc("POST /servers", func(w http.ResponseWriter, r *http.Request) {
		var spec Spec
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&spec); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := m.Create(spec); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, spec)
	})

	mux.HandleFunc("DELETE /servers/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := m.Remove(r.PathValue("id")); err != nil {
			writeError(w, statusFor(err), err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /servers/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, statusFor(err), err.Error())
			return
		}
		switch r.PathValue("action") {
		case "start":
			err = s.Start()
		case "stop":
			err = s.Stop()
		case "kill":
			err = s.Kill()
		default:
			writeError(w, http.StatusNotFound, "unknown action")
			return
		}
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]State{"state": s.State()})
	})

	mux.HandleFunc("GET /servers/{id}/console", func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, statusFor(err), err.Error())
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		history, lines, cancel := s.Subscribe()
		defer cancel()
		ctx, stop := context.WithCancel(r.Context())
		defer stop()

		// Anything the panel sends is a console command.
		go func() {
			defer stop()
			for {
				_, data, err := conn.Read(ctx)
				if err != nil {
					return
				}
				_ = s.Send(string(data))
			}
		}()

		for _, line := range history {
			if conn.Write(ctx, websocket.MessageText, []byte(line)) != nil {
				return
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case line := <-lines:
				if conn.Write(ctx, websocket.MessageText, []byte(line)) != nil {
					return
				}
			}
		}
	})

	return requireToken(token, mux)
}

func requireToken(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(strings.TrimSpace(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			writeError(w, http.StatusUnauthorized, "missing or wrong daemon token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func statusFor(err error) int {
	if errors.Is(err, ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusConflict
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
