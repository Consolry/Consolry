package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
)

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

var ErrNotFound = errors.New("server not found")

// Info is a server's spec plus its current state.
type Info struct {
	Spec
	State     State `json:"state"`
	StartedAt int64 `json:"startedAt"`
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
		m.servers[spec.ID] = newServer(spec, m.serverDir(spec.ID))
	}
	return m, nil
}

func (m *Manager) registry() string           { return filepath.Join(m.dir, "servers.json") }
func (m *Manager) serverDir(id string) string { return filepath.Join(m.dir, "servers", id) }

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
	m.servers[spec.ID] = newServer(spec, m.serverDir(spec.ID))
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
		list = append(list, Info{Spec: s.spec, State: s.State(), StartedAt: s.StartedAt()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// Remove forgets a stopped server. Its files stay on disk.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[id]
	if !ok {
		return ErrNotFound
	}
	if state := s.State(); state == StateRunning || state == StateStopping {
		return errors.New("stop the server before removing it")
	}
	delete(m.servers, id)
	return m.saveLocked()
}

// Shutdown stops every running server so none is left behind when the daemon exits.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.servers {
		_ = s.Kill()
	}
}
