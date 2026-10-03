package panel

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Consolry/Consolry/internal/daemon"
)

// fakePterodactyl answers the few client API calls the importer makes, for one Paper server.
func fakePterodactyl(t *testing.T) (*httptest.Server, *[]string) {
	var deleted []string
	var archive bytes.Buffer
	zipped := gzip.NewWriter(&archive)
	packed := tar.NewWriter(zipped)
	for name, body := range map[string]string{
		"server.jar":      "not really a jar",
		"logs/latest.log": "[12:00:00 INFO]: Starting minecraft server version 1.21.4\n",
		"world/level.dat": "a world",
		"../escape.txt":   "must not land outside the folder",
	} {
		_ = packed.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = packed.Write([]byte(body))
	}
	_ = packed.Close()
	_ = zipped.Close()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download/archive.tar.gz" {
			_, _ = w.Write(archive.Bytes())
			return
		}
		if r.Header.Get("Authorization") != "Bearer ptlc_good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/client":
			_, _ = w.Write([]byte(`{"data":[{"attributes":{"identifier":"abc123","name":"Survival SMP","docker_image":"ghcr.io/pterodactyl/yolks:java_21",
				"invocation":"java -Xms128M -jar server.jar","limits":{"memory":3072},
				"relationships":{"egg":{"attributes":{"name":"Paper"}}}}}]}`))
		case r.URL.Path == "/api/client/servers/abc123/startup":
			_, _ = w.Write([]byte(`{"data":[{"attributes":{"env_variable":"SERVER_JARFILE","server_value":"server.jar"}},{"attributes":{"env_variable":"MINECRAFT_VERSION","server_value":"latest"}}]}`))
		case r.URL.Path == "/api/client/servers/abc123/files/list":
			_, _ = w.Write([]byte(`{"data":[{"attributes":{"name":"server.jar"}},{"attributes":{"name":"world"}},{"attributes":{"name":"logs"}}]}`))
		case r.URL.Path == "/api/client/servers/abc123/files/compress":
			_, _ = w.Write([]byte(`{"attributes":{"name":"archive.tar.gz"}}`))
		case r.URL.Path == "/api/client/servers/abc123/files/download":
			_, _ = w.Write([]byte(`{"attributes":{"url":"` + server.URL + `/download/archive.tar.gz"}}`))
		case r.URL.Path == "/api/client/servers/abc123/files/delete":
			deleted = append(deleted, "archive.tar.gz")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server, &deleted
}

func TestImportFromPterodactyl(t *testing.T) {
	ptero, deleted := fakePterodactyl(t)
	defer ptero.Close()

	const token = "test-token"
	dataDir := t.TempDir()
	manager, err := daemon.NewManager(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown()
	node := httptest.NewServer(daemon.Handler(manager, token, "test"))
	defer node.Close()
	store, err := OpenStore(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	panel := httptest.NewServer(New(store, "test").Handler())
	defer panel.Close()

	admin := signedInClient(t, panel.URL)
	admin.do("POST", "/api/setup", map[string]string{"username": "admin", "password": "correct horse battery"}, nil)
	var created struct {
		ID int64 `json:"id"`
	}
	admin.do("POST", "/api/nodes", map[string]string{"name": "local", "url": node.URL, "token": token}, &created)

	if code := admin.do("POST", "/api/import/pterodactyl/servers", map[string]string{"url": ptero.URL, "key": "ptlc_bad"}, nil); code != http.StatusBadGateway {
		t.Fatalf("a wrong key got %d, want 502", code)
	}
	var listed []pteroServer
	if code := admin.do("POST", "/api/import/pterodactyl/servers", map[string]string{"url": ptero.URL, "key": "ptlc_good"}, &listed); code != http.StatusOK ||
		len(listed) != 1 || listed[0].Software != "paper" || listed[0].MemoryMB != 3072 {
		t.Fatalf("listing got %d: %+v", code, listed)
	}

	var result struct {
		ID    string   `json:"id"`
		Files int      `json:"files"`
		Notes []string `json:"notes"`
	}
	code := admin.do("POST", "/api/import/pterodactyl", map[string]any{"url": ptero.URL, "key": "ptlc_good", "identifier": "abc123", "nodeId": created.ID}, &result)
	if code != http.StatusCreated || result.Files != 3 {
		t.Fatalf("import got %d: %+v", code, result)
	}

	folder := filepath.Join(dataDir, "servers", result.ID)
	if data, err := os.ReadFile(filepath.Join(folder, "world", "level.dat")); err != nil || string(data) != "a world" {
		t.Fatalf("the world was not copied: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "servers", "escape.txt")); err == nil {
		t.Fatal("a file escaped the server's folder")
	}
	row, err := store.Server(result.ID)
	if err != nil || row.Kind != "minecraft" || row.Software != "paper" || row.MCVersion != "1.21.4" {
		t.Fatalf("imported server: %+v %v", row, err)
	}
	var info daemon.Info
	for _, item := range manager.List() {
		if item.ID == result.ID {
			info = item
		}
	}
	if info.Java != 21 || !strings.Contains(strings.Join(info.Args, " "), "-Xmx3072M") {
		t.Fatalf("start-up: java %d, args %v", info.Java, info.Args)
	}
	if len(*deleted) != 1 {
		t.Fatal("the archive was not removed from Pterodactyl afterwards")
	}
}
