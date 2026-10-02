// Package daemon runs game servers on one machine and exposes them to the panel.
package daemon

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

type State string

const (
	StateOffline State = "offline"
	StateRunning State = "running"
	// StateStarting is reported while the process runs but has not yet printed its ready line.
	StateStarting State = "starting"
	StateStopping State = "stopping"
	StateCrashed  State = "crashed"
)

// Spec is everything the daemon needs to know to run one server.
type Spec struct {
	ID          string   `json:"id"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	StopCommand string   `json:"stopCommand"`
	// Java is the oldest Java version this server can run on. When set and the command is
	// plain "java", the daemon picks a suitable Java itself. Zero means run the command as written.
	Java int `json:"java,omitempty"`
}

const (
	maxLines    = 2000
	stopTimeout = 30 * time.Second
)

// Server is one game server process and its console history.
type Server struct {
	spec    Spec
	dir     string
	javaDir string

	mu        sync.Mutex
	state     State
	startedAt time.Time

	// The last CPU reading, kept so the next one can report use since then.
	lastCPU    float64
	lastSample time.Time
	samplePid  int

	// ready is set once the server prints its ready line; lastLine is its newest output.
	ready    bool
	lastLine string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	lines    []string
	subs     map[chan string]struct{}
}

func newServer(spec Spec, dir, javaDir string) *Server {
	return &Server{spec: spec, dir: dir, javaDir: javaDir, state: StateOffline, subs: map[chan string]struct{}{}}
}

// minecraftReady matches the line a Minecraft server prints when the world has loaded.
var minecraftReady = regexp.MustCompile(`Done \([0-9.,]+s\)`)

var logPrefix = regexp.MustCompile(`^\[[^\]]*\]:? ?`)

func (s *Server) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateLocked()
}

// stateLocked is the state shown to the outside. A Java server counts as starting,
// not running, until it has printed its ready line.
func (s *Server) stateLocked() State {
	if s.state == StateRunning && s.spec.Java > 0 && !s.ready {
		return StateStarting
	}
	return s.state
}

// Progress is the newest line of output while the server is starting, for showing what it is doing.
func (s *Server) Progress() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stateLocked() != StateStarting {
		return ""
	}
	return s.lastLine
}

// StartedAt is when the running process was launched, as Unix seconds, or 0 if it isn't running.
func (s *Server) StartedAt() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.startedAt.IsZero() {
		return 0
	}
	return s.startedAt.Unix()
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil {
		return errors.New("server is already running")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}

	program := s.spec.Command
	if s.spec.Java > 0 && program == "java" {
		var err error
		if program, err = javaFor(s.javaDir, s.spec.Java); err != nil {
			s.state = StateCrashed
			s.appendLocked("[consolry] Could not start: " + err.Error())
			return err
		}
	}
	cmd := exec.Command(program, s.spec.Args...)
	cmd.Dir = s.dir
	prepare(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	// One pipe for both streams keeps output in the order the server wrote it.
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = w, w

	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		s.state = StateCrashed
		s.appendLocked("[consolry] Could not start: " + err.Error())
		return err
	}
	w.Close()

	s.cmd, s.stdin, s.state, s.startedAt = cmd, stdin, StateRunning, time.Now()
	s.ready, s.lastLine = false, ""
	s.appendLocked("[consolry] Server started")
	go s.watch(cmd, r)
	return nil
}

func (s *Server) watch(cmd *exec.Cmd, r *os.File) {
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			s.append(cleanLine(scanner.Text()))
		}
	}()

	_ = cmd.Wait()
	// A child process can keep the pipe open after the server exits, so don't wait on it forever.
	select {
	case <-drained:
	case <-time.After(2 * time.Second):
	}
	r.Close()

	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if code != 0 && s.state != StateStopping {
		s.state = StateCrashed
	} else {
		s.state = StateOffline
	}
	s.cmd, s.stdin, s.startedAt = nil, nil, time.Time{}
	s.appendLocked(fmt.Sprintf("[consolry] Server stopped (exit code %d)", code))
}

// colourCodes matches the terminal escape sequences servers use to colour their output.
var colourCodes = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// cleanLine prepares one line of server output for the console: no trailing carriage
// return, and no colour codes, which a browser would show as stray characters.
func cleanLine(line string) string {
	return colourCodes.ReplaceAllString(strings.TrimRight(line, "\r"), "")
}

// Stop asks the server to shut down cleanly, then kills it if it hasn't after stopTimeout.
func (s *Server) Stop() error {
	s.mu.Lock()
	if s.state != StateRunning {
		s.mu.Unlock()
		return errors.New("server is not running")
	}
	s.state = StateStopping
	cmd, stdin, stop := s.cmd, s.stdin, s.spec.StopCommand
	s.mu.Unlock()

	if stop == "" {
		return killTree(cmd)
	}
	if _, err := io.WriteString(stdin, stop+"\n"); err != nil {
		return killTree(cmd)
	}
	go func() {
		time.Sleep(stopTimeout)
		s.mu.Lock()
		still := s.cmd == cmd
		s.mu.Unlock()
		if still {
			_ = killTree(cmd)
		}
	}()
	return nil
}

func (s *Server) Kill() error {
	s.mu.Lock()
	cmd := s.cmd
	if cmd == nil {
		s.mu.Unlock()
		return errors.New("server is not running")
	}
	s.state = StateStopping
	s.mu.Unlock()
	return killTree(cmd)
}

// Send writes one console command to the server.
func (s *Server) Send(line string) error {
	s.mu.Lock()
	stdin := s.stdin
	s.mu.Unlock()
	if stdin == nil {
		return errors.New("server is not running")
	}
	_, err := io.WriteString(stdin, strings.TrimRight(line, "\r\n")+"\n")
	return err
}

// Subscribe returns the console history so far and a channel of new lines.
func (s *Server) Subscribe() (history []string, lines chan string, cancel func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lines = make(chan string, 256)
	s.subs[lines] = struct{}{}
	history = append([]string(nil), s.lines...)
	return history, lines, func() {
		s.mu.Lock()
		delete(s.subs, lines)
		s.mu.Unlock()
	}
}

func (s *Server) append(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appendLocked(line)
}

func (s *Server) appendLocked(line string) {
	if !s.ready && s.cmd != nil && !strings.HasPrefix(line, "[consolry]") {
		if minecraftReady.MatchString(line) {
			s.ready = true
		} else if text := strings.TrimSpace(logPrefix.ReplaceAllString(line, "")); text != "" && !strings.HasPrefix(text, "WARNING:") && !strings.HasPrefix(text, "- ") {
			if len(text) > 90 {
				text = text[:90]
			}
			s.lastLine = text
		}
	}
	s.lines = append(s.lines, line)
	if len(s.lines) > maxLines {
		s.lines = s.lines[len(s.lines)-maxLines:]
	}
	for sub := range s.subs {
		select {
		case sub <- line:
		default: // a slow viewer misses lines rather than stalling the server
		}
	}
}
