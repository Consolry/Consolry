package daemon

import (
	"os"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Consolry/Consolry/internal/testhelper"
)

func TestCleanLineIsAlwaysValidText(t *testing.T) {
	// A plugin printing "»" in the Windows encoding: one byte, 0xBB, which is not valid UTF-8.
	got := cleanLine("[05:27:12 INFO]: AxAuctions \xbb There is a new version available!")
	if !utf8.ValidString(got) {
		t.Fatalf("cleanLine returned invalid UTF-8: %q", got)
	}
	if want := "[05:27:12 INFO]: AxAuctions » There is a new version available!"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Proper UTF-8 must pass through untouched.
	if got := cleanLine("héllo » wörld ✓"); got != "héllo » wörld ✓" {
		t.Errorf("valid UTF-8 was changed to %q", got)
	}
}

func TestMain(m *testing.M) {
	testhelper.RunEchoIfRequested()
	os.Exit(m.Run())
}

func echoSpec(id string) Spec {
	return Spec{ID: id, Command: os.Args[0], Args: []string{testhelper.EchoArg}, StopCommand: "stop"}
}

func waitLine(t *testing.T, lines chan string, want string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case line := <-lines:
			if line == want {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for console line %q", want)
		}
	}
}

func waitState(t *testing.T, s *Server, want State) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if s.State() == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("state is %q, want %q", s.State(), want)
}

func TestCleanLineKeepsColoursOnly(t *testing.T) {
	// Colour codes stay; cursor movement, line clearing and the trailing carriage return go.
	raw := "[K[04:40:16 INFO]: [38;5;3mThere are [91m0[0m players online.[2J"
	got := cleanLine(raw)
	want := "[04:40:16 INFO]: [38;5;3mThere are [91m0[0m players online."
	if got != want {
		t.Errorf("cleanLine = %q, want %q", got, want)
	}
	if plain := Plain(got); plain != "[04:40:16 INFO]: There are 0 players online." {
		t.Errorf("Plain = %q", plain)
	}
}

func TestServerLifecycle(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Create(echoSpec("lifecycle")); err != nil {
		t.Fatal(err)
	}
	s, _ := m.Get("lifecycle")
	_, lines, cancel := s.Subscribe()
	defer cancel()

	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	waitLine(t, lines, "ready")
	if err := s.Start(); err == nil {
		t.Error("starting a running server should fail")
	}

	if err := s.Send("hello"); err != nil {
		t.Fatal(err)
	}
	waitLine(t, lines, "echo: hello")

	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	waitLine(t, lines, "bye")
	waitState(t, s, StateOffline)

	history, _, cancelHistory := s.Subscribe()
	cancelHistory()
	if len(history) < 4 {
		t.Errorf("history has %d lines, want the full session", len(history))
	}
}

func TestCrashIsReported(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	_ = m.Create(echoSpec("crasher"))
	s, _ := m.Get("crasher")
	_, lines, cancel := s.Subscribe()
	defer cancel()

	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	waitLine(t, lines, "ready")
	_ = s.Send("crash")
	waitState(t, s, StateCrashed)
}

func TestKill(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	_ = m.Create(echoSpec("killme"))
	s, _ := m.Get("killme")
	_, lines, cancel := s.Subscribe()
	defer cancel()

	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	waitLine(t, lines, "ready")
	if err := s.Kill(); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, StateOffline)
}

func TestRegistrySurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir)
	if err := m.Create(echoSpec("kept")); err != nil {
		t.Fatal(err)
	}
	if err := m.Create(Spec{ID: "Bad ID", Command: "x"}); err == nil {
		t.Error("an invalid id should be rejected")
	}

	reopened, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	list := reopened.List()
	if len(list) != 1 || list[0].ID != "kept" || list[0].State != StateOffline {
		t.Fatalf("unexpected registry after restart: %+v", list)
	}
	if err := reopened.Remove("kept"); err != nil {
		t.Fatal(err)
	}
	if len(reopened.List()) != 0 {
		t.Error("server should be gone after Remove")
	}
}

func TestStartingUntilReady(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	spec := echoSpec("slowstart")
	spec.Java = 21 // marks it as a server that announces when it is ready
	_ = m.Create(spec)
	s, _ := m.Get("slowstart")
	_, lines, cancel := s.Subscribe()
	defer cancel()

	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	waitLine(t, lines, "ready")
	if got := s.State(); got != StateStarting {
		t.Fatalf("state before the ready line is %q, want starting", got)
	}
	if got := s.Progress(); got != "ready" {
		t.Errorf("progress is %q, want the newest output line", got)
	}

	// The ready line is recognised even when the server colours it.
	_ = s.Send("[32mDone (1.234s)! For help, type \"help\"[0m")
	waitState(t, s, StateRunning)
	if got := s.Progress(); got != "" {
		t.Errorf("progress should be empty once running, got %q", got)
	}

	// A server that is still starting can be stopped like any other.
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	waitState(t, s, StateOffline)
}

func TestShutdownStopsServersCleanly(t *testing.T) {
	m, _ := NewManager(t.TempDir())
	_ = m.Create(echoSpec("closing"))
	s, _ := m.Get("closing")
	_, lines, cancel := s.Subscribe()
	defer cancel()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	waitLine(t, lines, "ready")

	m.Shutdown()

	// "bye" is what the test server prints when it is asked to stop, rather than being killed.
	waitLine(t, lines, "bye")
	if got := s.State(); got != StateOffline {
		t.Errorf("state after shutdown is %q, want offline", got)
	}
}
