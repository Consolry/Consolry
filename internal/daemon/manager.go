package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

var ErrNotFound = errors.New("server not found")

// Info is a server's spec plus its current state.
type Info struct {
	Spec
	State     State `json:"state"`
	StartedAt int64 `json:"startedAt"`
	// CPU is a percentage of the whole machine; Memory is in bytes.
	CPU    float64 `json:"cpu"`
	Memory uint64  `json:"memory"`
	// Progress is the server's newest output line while it is starting.
	Progress string `json:"progress,omitempty"`
}

// Manager owns every server on this machine and remembers them across restarts.
type Manager struct {
	dir string

	mu      sync.Mutex
	servers map[string]*Server
}

func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(filepath.Join(dir, "servers"), 0o755); err != nil {
		return nil, err
	}
	m := &Manager{dir: dir, servers: map[string]*Server{}}

	data, err := os.ReadFile(m.registry())
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	var specs []Spec
	if err := json.Unmarshal(data, &specs); err != nil {
		return nil, err
	}
	for _, spec := range specs {
		m.servers[spec.ID] = newServer(spec, m.serverDir(spec.ID), m.javaDir())
	}
	return m, nil
}

func (m *Manager) registry() string           { return filepath.Join(m.dir, "servers.json") }
func (m *Manager) serverDir(id string) string { return filepath.Join(m.dir, "servers", id) }
func (m *Manager) javaDir() string            { return filepath.Join(m.dir, "java") }

func (m *Manager) saveLocked() error {
	specs := make([]Spec, 0, len(m.servers))
	for _, s := range m.servers {
		specs = append(specs, s.spec)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].ID < specs[j].ID })
	data, err := json.MarshalIndent(specs, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.registry() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.registry())
}

func (m *Manager) Create(spec Spec) error {
	if !validID.MatchString(spec.ID) {
		return errors.New("id must be lowercase letters, digits and dashes")
	}
	if spec.Command == "" {
		return errors.New("a start command is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.servers[spec.ID]; exists {
		return errors.New("a server with that id already exists")
	}
	if err := os.MkdirAll(m.serverDir(spec.ID), 0o755); err != nil {
		return err
	}
	m.servers[spec.ID] = newServer(spec, m.serverDir(spec.ID), m.javaDir())
	return m.saveLocked()
}

func (m *Manager) Get(id string) (*Server, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (m *Manager) List() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := make([]Info, 0, len(m.servers))
	for _, s := range m.servers {
		info := Info{Spec: s.spec, State: s.State(), StartedAt: s.StartedAt()}
		info.CPU, info.Memory = s.Usage()
		info.Progress = s.Progress()
		list = append(list, info)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// Update changes how a stopped server is run. Its id and files stay the same.
func (m *Manager) Update(id string, spec Spec) error {
	if spec.Command == "" {
		return errors.New("a start command is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[id]
	if !ok {
		return ErrNotFound
	}
	s.mu.Lock()
	if s.cmd != nil {
		s.mu.Unlock()
		return errors.New("stop the server before changing how it starts")
	}
	spec.ID = id
	s.spec = spec
	s.mu.Unlock()
	return m.saveLocked()
}

// Remove forgets a stopped server. Its files stay on disk.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[id]
	if !ok {
		return ErrNotFound
	}
	if state := s.State(); state != StateOffline && state != StateCrashed {
		return errors.New("stop the server before removing it")
	}
	delete(m.servers, id)
	return m.saveLocked()
}

// shutdownGrace is how long a server gets to save and exit by itself before it is ended.
const shutdownGrace = 45 * time.Second

// Shutdown stops every running server so none is left behind when the daemon exits.
// Each is asked to stop cleanly first, so worlds are saved; only one that does not
// finish in time is ended by force.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	servers := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	m.mu.Unlock()

	var wait sync.WaitGroup
	for _, s := range servers {
		if state := s.State(); state == StateOffline || state == StateCrashed {
			continue
		}
		wait.Add(1)
		go func(s *Server) {
			defer wait.Done()
			if s.Stop() != nil {
				_ = s.Kill()
			}
			deadline := time.Now().Add(shutdownGrace)
			for time.Now().Before(deadline) {
				if state := s.State(); state == StateOffline || state == StateCrashed {
					return
				}
				time.Sleep(200 * time.Millisecond)
			}
			_ = s.Kill()
		}(s)
	}
	wait.Wait()
}
