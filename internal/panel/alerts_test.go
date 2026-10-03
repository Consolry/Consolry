package panel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DinoNaedYT/Consolry/internal/daemon"
	"github.com/DinoNaedYT/Consolry/internal/testhelper"
)

func TestCheckAlerts(t *testing.T) {
	good, err := checkAlerts(Alerts{Discord: " https://discord.com/api/webhooks/1/abc ", Email: "a@example.com, B <b@example.com>", Events: []string{"crash", "nope"}}, alertEvents)
	if err != nil || good.Email != "a@example.com, b@example.com" || len(good.Events) != 1 {
		t.Fatalf("got %+v, %v", good, err)
	}
	for _, webhook := range []string{"http://discord.com/api/webhooks/1/a", "https://example.com/api/webhooks/1/a", "https://discord.com/other", "https://127.0.0.1/api/webhooks/1"} {
		if _, err := checkAlerts(Alerts{Discord: webhook}, alertEvents); err == nil {
			t.Errorf("webhook %q should be refused", webhook)
		}
	}
	if _, err := checkAlerts(Alerts{Email: "not an address"}, alertEvents); err == nil {
		t.Error("a bad email address should be refused")
	}
}

// A crash is noticed by the watcher and sent to the server's Discord webhook,
// with the crash explainer's reading when it has one.
func TestCrashAlert(t *testing.T) {
	received := make(chan string, 4)
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Content string `json:"content"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		received <- body.Content
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discord.Close()

	const token = "test-token"
	manager, err := daemon.NewManager(t.TempDir())
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
	app := New(store, "test")
	panel := httptest.NewServer(app.Handler())
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
	admin.do("POST", "/api/servers", map[string]any{
		"name": "Crashy", "nodeId": created.ID,
		"startCommand": `"` + os.Args[0] + `" ` + testhelper.EchoArg,
	}, &server)

	// Stored directly: the page only accepts discord.com, and the test stands in for it.
	if err := store.SetAlerts(server.ID, Alerts{Discord: discord.URL + "/api/webhooks/1/x", Events: []string{"crash", "start"}}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.StartWatching(ctx)
	time.Sleep(300 * time.Millisecond) // the first look only records what is there

	admin.do("POST", "/api/servers/"+server.ID+"/power", map[string]string{"action": "start"}, nil)
	app.watchOnce(ctx)
	// The echo helper exits with an error when told to, which the daemon reports as a crash.
	if s, err := manager.Get(server.ID); err == nil {
		_ = s.Send("crash")
	}
	deadline := time.Now().Add(10 * time.Second)
	var messages []string
	for time.Now().Before(deadline) {
		app.watchOnce(ctx)
		select {
		case message := <-received:
			messages = append(messages, message)
		case <-time.After(200 * time.Millisecond):
		}
		if strings.Contains(strings.Join(messages, "\n"), "crashed") {
			break
		}
	}
	all := strings.Join(messages, "\n")
	if !strings.Contains(all, "Crashy is running") || !strings.Contains(all, "Crashy crashed") {
		t.Fatalf("alerts received: %q", messages)
	}
}

func signedInClient(t *testing.T, base string) client {
	jar, _ := cookiejar.New(nil)
	return client{t: t, base: base, http: &http.Client{Jar: jar}}
}
