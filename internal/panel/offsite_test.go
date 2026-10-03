package panel

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"github.com/DinoNaedYT/Consolry/internal/daemon"
	"github.com/DinoNaedYT/Consolry/internal/testhelper"
)

// A backup is copied to S3-compatible storage, listed, and restored from there.
func TestOffsiteBackups(t *testing.T) {
	backend := s3mem.New()
	if err := backend.CreateBucket("worlds"); err != nil {
		t.Fatal(err)
	}
	s3 := httptest.NewServer(gofakes3.New(backend).Server())
	defer s3.Close()

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
	var server struct {
		ID string `json:"id"`
	}
	admin.do("POST", "/api/servers", map[string]any{"name": "World", "nodeId": created.ID, "startCommand": `"` + os.Args[0] + `" ` + testhelper.EchoArg}, &server)
	base := "/api/servers/" + server.ID

	// Turning it on before storage is set up is refused with a reason.
	if code := admin.do("POST", base+"/offsite", map[string]any{"enabled": true}, nil); code != http.StatusConflict {
		t.Fatalf("enabling without storage got %d, want 409", code)
	}
	storage := map[string]any{"endpoint": s3.URL, "region": "us-east-1", "bucket": "worlds", "accessKey": "key", "secret": "secret", "keep": 2}
	if code := admin.do("POST", "/api/storage", map[string]any{"endpoint": s3.URL, "bucket": "missing", "accessKey": "key", "secret": "secret", "keep": 2}, nil); code != http.StatusBadRequest {
		t.Fatalf("a bucket that does not exist got %d, want 400", code)
	}
	if code := admin.do("POST", "/api/storage", storage, nil); code != http.StatusOK {
		t.Fatalf("saving storage got %d, want 200", code)
	}
	admin.do("POST", base+"/offsite", map[string]any{"enabled": true}, nil)

	// Three backups by hand; only the newest two are kept off-site.
	world := filepath.Join(dataDir, "servers", server.ID, "world.txt")
	_ = os.WriteFile(world, []byte("version 1"), 0o644)
	var listing struct {
		Enabled bool          `json:"enabled"`
		Copies  []offsiteCopy `json:"copies"`
	}
	for i := 0; i < 3; i++ {
		if code := admin.do("POST", base+"/node/backups", map[string]any{}, nil); code != http.StatusCreated {
			t.Fatalf("backup got %d", code)
		}
		want := min(i+1, 2)
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			admin.do("GET", base+"/offsite", nil, &listing)
			if len(listing.Copies) == want && (i < 2 || listing.Copies[0].Name > listing.Copies[1].Name) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if len(listing.Copies) != want {
			t.Fatalf("after backup %d there are %d copies off-site, want %d", i+1, len(listing.Copies), want)
		}
		time.Sleep(1100 * time.Millisecond) // backups are named by the second
	}

	// Lose the local backups and the world, then restore from off-site.
	newest := listing.Copies[0].Name
	_ = os.RemoveAll(filepath.Join(dataDir, "backups", server.ID))
	_ = os.WriteFile(world, []byte("ruined"), 0o644)
	if code := admin.do("POST", base+"/offsite/"+newest+"/restore", map[string]any{}, nil); code != http.StatusNoContent {
		t.Fatalf("restore from off-site got %d, want 204", code)
	}
	if data, _ := os.ReadFile(world); string(data) != "version 1" {
		t.Fatalf("after the restore the world says %q", data)
	}

	// Download goes straight to the storage with a short-lived link.
	noRedirect := &http.Client{Jar: admin.http.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := noRedirect.Get(panel.URL + base + "/offsite/" + newest)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") == "" {
		t.Fatalf("download got %d to %q", res.StatusCode, res.Header.Get("Location"))
	}
}
