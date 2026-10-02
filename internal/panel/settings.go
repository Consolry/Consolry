package panel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/DinoNaedYT/Consolry/internal/minecraft"
)

// autoBackupsKept is how many before-a-change backups stay on disk per server.
const autoBackupsKept = 5

// backupFirst takes a safety backup before something changes the server's files.
// A backup made in the last few minutes is reused, so a run of changes copies the world once.
func backupFirst(ctx context.Context, node Node, id string) error {
	path := fmt.Sprintf("/servers/%s/backups?kind=auto&keep=%d", id, autoBackupsKept)
	if err := node.slow(ctx, http.MethodPost, path, nil, nil); err != nil {
		return fmt.Errorf("nothing was changed, because the safety backup failed: %w", err)
	}
	return nil
}

// ensureJava makes sure the node can run a server needing the given Java version, installing
// a private copy if the machine has none. It returns a sentence for the user if that fails.
func ensureJava(ctx context.Context, node Node, needed int) string {
	if needed == 0 {
		return ""
	}
	if err := node.slow(ctx, http.MethodPost, "/java/"+strconv.Itoa(needed), nil, nil); err != nil {
		return fmt.Sprintf("This server needs Java %d or newer, and it could not be installed automatically (%v). Install it on this machine before starting the server.", needed, err)
	}
	return ""
}

// memoryArgs replaces the -Xms and -Xmx values in a Java command's arguments.
func memoryArgs(args []string, megabytes int) []string {
	memory := strconv.Itoa(megabytes) + "M"
	out := []string{"-Xms" + memory, "-Xmx" + memory}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-Xms") && !strings.HasPrefix(arg, "-Xmx") {
			out = append(out, arg)
		}
	}
	return out
}

func (a *App) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name         *string `json:"name"`
		MemoryMB     *int    `json:"memoryMb"`
		StartCommand *string `json:"startCommand"`
		StopCommand  *string `json:"stopCommand"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}

	if input.MemoryMB != nil || input.StartCommand != nil || input.StopCommand != nil {
		spec, err := serverState(r.Context(), node, row.ID)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		if input.StartCommand != nil {
			command, args := splitCommand(*input.StartCommand)
			if command == "" {
				writeError(w, http.StatusBadRequest, "a start command is required")
				return
			}
			spec.Command, spec.Args = command, args
		}
		if input.StopCommand != nil {
			spec.StopCommand = strings.TrimSpace(*input.StopCommand)
		}
		if input.MemoryMB != nil {
			if *input.MemoryMB < 512 || *input.MemoryMB > 262144 {
				writeError(w, http.StatusBadRequest, "memory must be between 512 MB and 256 GB")
				return
			}
			spec.Args = memoryArgs(spec.Args, *input.MemoryMB)
		}
		spec.State, spec.StartedAt = "", 0
		if err := node.call(r.Context(), http.MethodPut, "/servers/"+row.ID, spec, nil); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "give the server a name")
			return
		}
		if err := a.store.RenameServer(row.ID, name); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSwitchVersion moves a Minecraft server to another version or another kind of server software.
func (a *App) handleSwitchVersion(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Software string `json:"software"`
		Version  string `json:"version"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	row, node, _, ok := a.minecraftServer(w, r)
	if !ok {
		return
	}
	if _, known := minecraft.FindSoftware(input.Software); !known {
		writeError(w, http.StatusBadRequest, "choose Paper, Purpur or Fabric")
		return
	}
	spec, err := serverState(r.Context(), node, row.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if spec.State == "running" || spec.State == "stopping" {
		writeError(w, http.StatusConflict, "stop the server before switching versions")
		return
	}
	download, err := minecraft.Resolve(r.Context(), input.Software, input.Version)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not find that server version: "+err.Error())
		return
	}
	if err := backupFirst(r.Context(), node, row.ID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	fetch := map[string]string{"url": download.URL, "path": "server.jar", "sha256": download.SHA256}
	if err := node.slow(r.Context(), http.MethodPost, "/servers/"+row.ID+"/files/fetch", fetch, nil); err != nil {
		writeError(w, http.StatusBadGateway, "could not download the server: "+err.Error())
		return
	}

	spec.Java, spec.State, spec.StartedAt = download.JavaMin, "", 0
	if err := node.call(r.Context(), http.MethodPut, "/servers/"+row.ID, spec, nil); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := a.store.SetServerVersion(row.ID, input.Software, input.Version); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"warning": ensureJava(r.Context(), node, download.JavaMin)})
}

// EnsureLocalNode registers the panel's built-in daemon as a node, so a fresh install
// has somewhere to run servers without the user entering an address and token.
func (s *Store) EnsureLocalNode(address, token string) error {
	var id int64
	var current string
	err := s.db.QueryRow(`SELECT id, token FROM nodes WHERE url = ?`, address).Scan(&id, &current)
	if err == nil {
		if current != token {
			_, err = s.db.Exec(`UPDATE nodes SET token = ? WHERE id = ?`, token, id)
		}
		return err
	}
	_, err = s.CreateNode("This machine", address, token)
	return err
}

func (s *Store) RenameServer(id, name string) error {
	_, err := s.db.Exec(`UPDATE servers SET name = ? WHERE id = ?`, name, id)
	return err
}

func (s *Store) SetServerVersion(id, software, version string) error {
	result, err := s.db.Exec(`UPDATE servers SET software = ?, mc_version = ? WHERE id = ?`, software, version, id)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return errors.New("server not found")
	}
	return nil
}
