// Package daemon runs game servers on one machine and exposes them to the panel.
package daemon

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type State string

const (
	StateOffline  State = "offline"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateCrashed  State = "crashed"
)

// Spec is everything the daemon needs to know to run one server.
type Spec struct {
	ID          string   `json:"id"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	StopCommand string   `json:"stopCommand"`
}

const (
	maxLines    = 2000
	stopTimeout = 30 * time.Second
)

// Server is one game server process and its console history.
type Server struct {
	spec Spec
	dir  string

	mu        sync.Mutex
	state     State
	startedAt time.Time
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	lines     []string
	subs      map[chan string]struct{}
}

func newServer(spec Spec, dir string) *Server {
	return &Server{spec: spec, dir: dir, state: StateOffline, subs: map[chan string]struct{}{}}
}

func (s *Server) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
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

	cmd := exec.Command(s.spec.Command, s.spec.Args...)
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
			s.append(strings.TrimRight(scanner.Text(), "\r"))
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
