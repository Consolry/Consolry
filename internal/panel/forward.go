package panel

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Forward records that a server's port has been opened on the router.
type Forward struct {
	Port       int    `json:"port"`
	ExternalIP string `json:"externalIp"`
	// Reachable is false when the router's own address is not public, so the port is open
	// on the router but people outside still cannot connect.
	Reachable bool `json:"reachable"`
}

// Forwarding returns the port forward recorded for a server, or nil if there is none.
func (s *Store) Forwarding(serverID string) *Forward {
	var raw string
	if s.db.QueryRow(`SELECT forward FROM servers WHERE id = ?`, serverID).Scan(&raw) != nil || raw == "" {
		return nil
	}
	var forward Forward
	if json.Unmarshal([]byte(raw), &forward) != nil {
		return nil
	}
	return &forward
}

func (s *Store) SetForwarding(serverID string, forward *Forward) error {
	raw := ""
	if forward != nil {
		data, _ := json.Marshal(forward)
		raw = string(data)
	}
	_, err := s.db.Exec(`UPDATE servers SET forward = ? WHERE id = ?`, raw, serverID)
	return err
}

// serverPort reads a Minecraft server's port from its settings file, defaulting to Minecraft's own.
func serverPort(ctx context.Context, node Node, id string) int {
	if value, err := strconv.Atoi(property(readServerFile(ctx, node, id, "server.properties"), "server-port")); err == nil && value > 0 {
		return value
	}
	return 25565
}

// openPort asks the node's router to forward a port and records the result.
func (a *App) openPort(ctx context.Context, node Node, id string, port int) (*Forward, error) {
	var forward Forward
	if err := node.slow(ctx, http.MethodPost, "/network/forward", map[string]int{"port": port}, &forward); err != nil {
		return nil, err
	}
	return &forward, a.store.SetForwarding(id, &forward)
}

func (a *App) handleForward(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	row, node, _, ok := a.minecraftServer(w, r)
	if !ok {
		return
	}

	if !input.Enabled {
		if current := a.store.Forwarding(row.ID); current != nil {
			// Forget it even if the router cannot be reached, so the panel stops reopening it.
			_ = node.slow(r.Context(), http.MethodDelete, "/network/forward?port="+strconv.Itoa(current.Port), nil, nil)
		}
		if err := a.store.SetForwarding(row.ID, nil); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.log(r, row.ID, "Closed the port on the router")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	port := serverPort(r.Context(), node, row.ID)
	// A forward left over from a different port would stay open for no reason.
	if current := a.store.Forwarding(row.ID); current != nil && current.Port != port {
		_ = node.slow(r.Context(), http.MethodDelete, "/network/forward?port="+strconv.Itoa(current.Port), nil, nil)
	}
	forward, err := a.openPort(r.Context(), node, row.ID, port)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	a.log(r, row.ID, "Opened port "+strconv.Itoa(port)+" on the router")
	writeJSON(w, http.StatusOK, forward)
}

// refreshForward reopens a server's port when it starts. Routers forget forwards when they
// restart, and some only grant them for a limited time.
func (a *App) refreshForward(node Node, row ServerRow) {
	current := a.store.Forwarding(row.ID)
	if current == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		port := serverPort(ctx, node, row.ID)
		if port != current.Port {
			_ = node.slow(ctx, http.MethodDelete, "/network/forward?port="+strconv.Itoa(current.Port), nil, nil)
		}
		if _, err := a.openPort(ctx, node, row.ID, port); err != nil {
			a.store.Log(row.ID, "consolry", "Could not reopen the port on the router: "+err.Error())
		}
	}()
}
