package daemon

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type api struct {
	t    *testing.T
	base string
}

func (a api) do(method, path, body string) (int, string) {
	a.t.Helper()
	req, _ := http.NewRequest(method, a.base+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(data)
}

func newAPI(t *testing.T) (api, *Manager, string) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Create(echoSpec("files")); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Handler(m, "secret", "test"))
	t.Cleanup(server.Close)
	return api{t: t, base: server.URL}, m, dir
}

func TestFilesAPI(t *testing.T) {
	a, _, dir := newAPI(t)

	if code, _ := a.do("PUT", "/servers/files/files/content?path=plugins/config.yml", "motd: hello\n"); code != http.StatusNoContent {
		t.Fatalf("write got %d", code)
	}
	if code, body := a.do("GET", "/servers/files/files/content?path=plugins/config.yml", ""); code != 200 || body != "motd: hello\n" {
		t.Fatalf("read got %d %q", code, body)
	}

	_, listing := a.do("GET", "/servers/files/files?path=", "")
	var entries []fileEntry
	if err := json.Unmarshal([]byte(listing), &entries); err != nil || len(entries) != 1 || entries[0].Name != "plugins" || !entries[0].Dir {
		t.Fatalf("unexpected listing: %s", listing)
	}

	if code, _ := a.do("POST", "/servers/files/files/rename", `{"from":"plugins/config.yml","to":"plugins/settings.yml"}`); code != http.StatusNoContent {
		t.Fatalf("rename got %d", code)
	}
	if code, _ := a.do("POST", "/servers/files/files/mkdir", `{"path":"world/region"}`); code != http.StatusNoContent {
		t.Fatalf("mkdir got %d", code)
	}
	if code, _ := a.do("DELETE", "/servers/files/files?path=plugins", ""); code != http.StatusNoContent {
		t.Fatalf("delete got %d", code)
	}
	if code, _ := a.do("GET", "/servers/files/files/content?path=plugins/settings.yml", ""); code != http.StatusNotFound {
		t.Fatalf("reading a deleted file got %d, want 404", code)
	}
	if code, _ := a.do("DELETE", "/servers/files/files?path=", ""); code != http.StatusBadRequest {
		t.Fatalf("deleting the server folder itself got %d, want 400", code)
	}
	if code, _ := a.do("POST", "/servers/files/files/fetch", `{"url":"http://example.com/x.jar","path":"x.jar"}`); code != http.StatusBadRequest {
		t.Fatalf("a plain http download got %d, want 400", code)
	}

	// Nothing outside the server's own folder may be read or written.
	secret := filepath.Join(dir, "token")
	if err := os.WriteFile(secret, []byte("do not leak"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, attempt := range []string{"../../token", "..%2F..%2Ftoken", "..\\..\\token", "/../../token"} {
		if code, body := a.do("GET", "/servers/files/files/content?path="+attempt, ""); code == 200 || strings.Contains(body, "do not leak") {
			t.Errorf("path %q escaped the server folder: %d %q", attempt, code, body)
		}
	}
	a.do("PUT", "/servers/files/files/content?path=../../escaped.txt", "x")
	if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); err == nil {
		t.Error("a write escaped the server folder")
	}
}

func TestBackupAndRestore(t *testing.T) {
	a, _, _ := newAPI(t)
	a.do("PUT", "/servers/files/files/content?path=world/level.dat", "original world")
	a.do("PUT", "/servers/files/files/content?path=server.properties", "motd=before")

	code, created := a.do("POST", "/servers/files/backups", "")
	var backup backupInfo
	if err := json.Unmarshal([]byte(created), &backup); code != http.StatusCreated || err != nil || backup.Size == 0 {
		t.Fatalf("backup got %d %s", code, created)
	}

	// Change and add files, then restore: the server folder should match the backup exactly.
	a.do("PUT", "/servers/files/files/content?path=world/level.dat", "griefed world")
	a.do("PUT", "/servers/files/files/content?path=added-later.txt", "x")
	if code, body := a.do("POST", "/servers/files/backups/"+backup.Name+"/restore", ""); code != http.StatusNoContent {
		t.Fatalf("restore got %d %s", code, body)
	}
	if _, body := a.do("GET", "/servers/files/files/content?path=world/level.dat", ""); body != "original world" {
		t.Errorf("restored file is %q", body)
	}
	if code, _ := a.do("GET", "/servers/files/files/content?path=added-later.txt", ""); code != http.StatusNotFound {
		t.Errorf("a file added after the backup should be gone after restore, got %d", code)
	}

	_, listing := a.do("GET", "/servers/files/backups", "")
	if !strings.Contains(listing, backup.Name) {
		t.Errorf("backup missing from list: %s", listing)
	}
	if code, _ := a.do("GET", "/servers/files/backups/..%2F..%2Ftoken", ""); code == 200 {
		t.Error("a backup name must not reach other files")
	}
	if code, _ := a.do("DELETE", "/servers/files/backups/"+backup.Name, ""); code != http.StatusNoContent {
		t.Errorf("delete backup got %d", code)
	}
}
