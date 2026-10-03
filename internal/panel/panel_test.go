package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Consolry/Consolry/internal/daemon"
	"github.com/Consolry/Consolry/internal/testhelper"
)

func TestMain(m *testing.M) {
	testhelper.RunEchoIfRequested()
	os.Exit(m.Run())
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

// do sends a JSON request to the panel and decodes the reply into out, returning the status code.
func (c client) do(method, path string, body, out any) int {
	c.t.Helper()
	var reader *bytes.Reader = bytes.NewReader(nil)
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, c.base+path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		_ = json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode
}

func TestPanelEndToEnd(t *testing.T) {
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
	panel := httptest.NewServer(New(store, "test").Handler())
	defer panel.Close()

	jar, _ := cookiejar.New(nil)
	c := client{t: t, base: panel.URL, http: &http.Client{Jar: jar}}
	stranger := client{t: t, base: panel.URL, http: &http.Client{}}

	// First run: the panel asks for setup, and nothing works without signing in.
	var state struct {
		SetupNeeded bool  `json:"setupNeeded"`
		User        *User `json:"user"`
	}
	c.do("GET", "/api/state", nil, &state)
	if !state.SetupNeeded {
		t.Fatal("a fresh panel should need setup")
	}
	if code := stranger.do("GET", "/api/servers", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request got %d, want 401", code)
	}
	if code := c.do("POST", "/api/setup", map[string]string{"username": "admin", "password": "short"}, nil); code != http.StatusBadRequest {
		t.Fatalf("short password got %d, want 400", code)
	}
	if code := c.do("POST", "/api/setup", map[string]string{"username": "admin", "password": "correct horse battery"}, nil); code != http.StatusCreated {
		t.Fatalf("setup got %d, want 201", code)
	}
	if code := stranger.do("POST", "/api/setup", map[string]string{"username": "intruder", "password": "another long password"}, nil); code != http.StatusConflict {
		t.Fatalf("second setup got %d, want 409", code)
	}
	if code := stranger.do("POST", "/api/login", map[string]string{"username": "admin", "password": "wrong password!"}, nil); code != http.StatusUnauthorized {
		t.Fatalf("wrong password got %d, want 401", code)
	}

	// A node is only accepted with the right token.
	if code := c.do("POST", "/api/nodes", map[string]string{"name": "local", "url": node.URL, "token": "wrong"}, nil); code != http.StatusBadGateway {
		t.Fatalf("wrong node token got %d, want 502", code)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if code := c.do("POST", "/api/nodes", map[string]string{"name": "local", "url": node.URL, "token": token}, &created); code != http.StatusCreated {
		t.Fatalf("add node got %d, want 201", code)
	}

	var server struct {
		ID string `json:"id"`
	}
	code := c.do("POST", "/api/servers", map[string]any{
		"name":         "My Test Server",
		"nodeId":       created.ID,
		"startCommand": `"` + os.Args[0] + `" ` + testhelper.EchoArg,
		"stopCommand":  "stop",
	}, &server)
	if code != http.StatusCreated || !strings.HasPrefix(server.ID, "my-test-server-") {
		t.Fatalf("create server got %d with id %q", code, server.ID)
	}

	// The console reaches the browser through the panel, in both directions.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	header := http.Header{}
	for _, cookie := range jar.Cookies(mustURL(t, panel.URL)) {
		header.Add("Cookie", cookie.String())
	}
	wsURL := strings.Replace(panel.URL, "http", "ws", 1) + "/api/servers/" + server.ID + "/console"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("console dial: %v", err)
	}
	defer conn.CloseNow()
	expect := func(want string) {
		t.Helper()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("waiting for %q: %v", want, err)
			}
			if string(data) == want {
				return
			}
		}
	}

	if code := c.do("POST", "/api/servers/"+server.ID+"/power", map[string]string{"action": "start"}, nil); code != http.StatusOK {
		t.Fatalf("start got %d", code)
	}
	expect("ready")
	if err := conn.Write(ctx, websocket.MessageText, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	expect("echo: hello")

	if got := stateOf(c, server.ID); got != "running" {
		t.Fatalf("state is %q, want running", got)
	}
	if code := c.do("DELETE", "/api/servers/"+server.ID, nil, nil); code != http.StatusConflict {
		t.Fatalf("deleting a running server got %d, want 409", code)
	}

	if code := c.do("POST", "/api/servers/"+server.ID+"/power", map[string]string{"action": "stop"}, nil); code != http.StatusOK {
		t.Fatalf("stop got %d", code)
	}
	expect("bye")
	deadline := time.Now().Add(10 * time.Second)
	for stateOf(c, server.ID) != "offline" {
		if time.Now().After(deadline) {
			t.Fatalf("server never went offline, state %q", stateOf(c, server.ID))
		}
		time.Sleep(50 * time.Millisecond)
	}

	if code := c.do("DELETE", "/api/servers/"+server.ID, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete got %d, want 204", code)
	}

	// A console connection without a session is refused.
	if _, _, err := websocket.Dial(ctx, wsURL, nil); err == nil {
		t.Error("console should refuse a connection with no session")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func stateOf(c client, id string) string {
	var servers []struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	c.do("GET", "/api/servers", nil, &servers)
	for _, server := range servers {
		if server.ID == id {
			return server.State
		}
	}
	return "missing"
}

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		line    string
		command string
		args    []string
	}{
		{"java -Xmx2G -jar server.jar nogui", "java", []string{"-Xmx2G", "-jar", "server.jar", "nogui"}},
		{`"C:\Program Files\Java\bin\java.exe" -jar "my server.jar"`, `C:\Program Files\Java\bin\java.exe`, []string{"-jar", "my server.jar"}},
		{"  spaced   out  ", "spaced", []string{"out"}},
		{`run ""`, "run", []string{""}},
		{"", "", nil},
	}
	for _, c := range cases {
		command, args := splitCommand(c.line)
		if command != c.command || !reflect.DeepEqual(args, c.args) {
			t.Errorf("splitCommand(%q) = %q %q, want %q %q", c.line, command, args, c.command, c.args)
		}
	}
}
