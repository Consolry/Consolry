package daemon

import (
	"net"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// usage is what a server's process, and anything it launched, is using right now.
type usage struct {
	cpuSeconds float64
	memory     uint64
}

// measure adds up CPU time and memory for a process and all its descendants.
func measure(pid int32, into *usage, depth int) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return
	}
	if times, err := p.Times(); err == nil {
		into.cpuSeconds += times.User + times.System
	}
	if memory, err := p.MemoryInfo(); err == nil && memory != nil {
		into.memory += memory.RSS
	}
	if depth > 4 {
		return
	}
	children, _ := p.Children()
	for _, child := range children {
		measure(child.Pid, into, depth+1)
	}
}

// Usage reports the server's CPU use, as a percentage of the whole machine since the
// last call, and its memory in bytes. Both are zero when it is not running.
func (s *Server) Usage() (cpu float64, memory uint64) {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return 0, 0
	}

	var now usage
	measure(int32(cmd.Process.Pid), &now, 0)
	at := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lastSample.IsZero() && s.samplePid == cmd.Process.Pid {
		if elapsed := at.Sub(s.lastSample).Seconds(); elapsed > 0 && now.cpuSeconds >= s.lastCPU {
			cpu = (now.cpuSeconds - s.lastCPU) / elapsed / float64(runtime.NumCPU()) * 100
		}
	}
	s.lastCPU, s.lastSample, s.samplePid = now.cpuSeconds, at, cmd.Process.Pid
	if cpu > 100 {
		cpu = 100
	}
	return cpu, now.memory
}

// lanAddresses lists this machine's addresses on the local network.
func lanAddresses() []string {
	found := []string{}
	addresses, _ := net.InterfaceAddrs()
	for _, address := range addresses {
		network, ok := address.(*net.IPNet)
		if !ok {
			continue
		}
		ip := network.IP.To4()
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || !ip.IsPrivate() {
			continue
		}
		found = append(found, ip.String())
	}
	return found
}

func registerNetwork(mux *http.ServeMux) {
	// network reports this machine's local addresses and whether something is listening on a port.
	mux.HandleFunc("GET /network", func(w http.ResponseWriter, r *http.Request) {
		port, err := strconv.Atoi(r.URL.Query().Get("port"))
		listening := false
		if err == nil && port > 0 && port < 65536 {
			if conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 400*time.Millisecond); err == nil {
				conn.Close()
				listening = true
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"addresses": lanAddresses(), "listening": listening, "cores": runtime.NumCPU()})
	})
}
