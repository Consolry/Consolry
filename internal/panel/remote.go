package panel

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/DinoNaedYT/Consolry/internal/daemon"
)

// Where the panel can be opened from. It always answers on the machine it runs on.
const (
	remoteOff      = "off"      // only this machine
	remoteNetwork  = "network"  // other devices on the same home network
	remoteInternet = "internet" // anywhere, through a port opened on the router
)

// RemoteStatus is what the panel shows about reaching it from other devices.
type RemoteStatus struct {
	Mode string `json:"mode"`
	Port int    `json:"port"`
	// NetworkAddress is the address other devices on the home network use.
	NetworkAddress string `json:"networkAddress"`
	// InternetAddress is the address to use from anywhere else, once the router has opened the port.
	InternetAddress string `json:"internetAddress"`
	// Problem says, in a sentence, why the chosen mode is not fully working.
	Problem string `json:"problem"`
}

// remoteAccess opens the panel to other devices: a second listener on this machine's
// network address and, for the internet, a port forwarded on the router.
type remoteAccess struct {
	mu      sync.Mutex
	handler http.Handler
	port    int
	// fixed is true when the panel was started listening on every address already,
	// so there is nothing to open or close here.
	fixed bool

	server    *http.Server
	bound     string
	forwarded bool
	state     RemoteStatus
}

// lanAddress is this machine's address on its local network.
func lanAddress() (string, error) {
	// No packet is sent: this only asks the system which address it would use to reach the internet.
	conn, err := net.Dial("udp4", "192.0.2.1:9")
	if err != nil {
		return "", errors.New("this machine does not seem to be connected to a network")
	}
	defer conn.Close()
	ip := conn.LocalAddr().(*net.UDPAddr).IP
	if ip.IsLoopback() || ip.IsUnspecified() {
		return "", errors.New("this machine does not seem to be connected to a network")
	}
	return ip.String(), nil
}

func (ra *remoteAccess) status() RemoteStatus {
	ra.mu.Lock()
	defer ra.mu.Unlock()
	state := ra.state
	if state.Mode == "" {
		state.Mode = remoteOff
	}
	state.Port = ra.port
	return state
}

// apply switches to a mode. It is safe to call again with the same mode, which
// re-checks the network address and asks the router again.
func (ra *remoteAccess) apply(ctx context.Context, mode string) {
	ra.mu.Lock()
	defer ra.mu.Unlock()
	if ra.handler == nil {
		return
	}
	state := RemoteStatus{Mode: mode}

	if mode != remoteInternet && ra.forwarded {
		_ = daemon.ClosePort(ctx, ra.port)
		ra.forwarded = false
	}
	if mode == remoteOff {
		ra.stopListening()
		ra.state = state
		return
	}

	address, err := lanAddress()
	if err != nil {
		ra.stopListening()
		state.Problem = err.Error()
		ra.state = state
		return
	}
	if !ra.fixed && ra.bound != address {
		ra.stopListening()
		listener, err := net.Listen("tcp", net.JoinHostPort(address, strconv.Itoa(ra.port)))
		if err != nil {
			state.Problem = "Could not listen on " + address + ": " + err.Error()
			ra.state = state
			return
		}
		ra.server = &http.Server{Handler: ra.handler, ReadHeaderTimeout: 10 * time.Second}
		ra.bound = address
		go func(server *http.Server) {
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("remote access stopped: %v", err)
			}
		}(ra.server)
	}
	state.NetworkAddress = "http://" + net.JoinHostPort(address, strconv.Itoa(ra.port))

	if mode == remoteInternet {
		external, reachable, err := daemon.OpenPort(ctx, ra.port)
		switch {
		case err != nil:
			state.Problem = "The panel is open on your home network, but not to the internet: " + err.Error()
		case !reachable:
			ra.forwarded = true
			state.Problem = "Your router opened the port, but your internet provider shares one public address between several customers, so the panel cannot be reached from outside your home."
		default:
			ra.forwarded = true
			state.InternetAddress = "http://" + net.JoinHostPort(external, strconv.Itoa(ra.port))
		}
	}
	ra.state = state
}

func (ra *remoteAccess) stopListening() {
	if ra.server != nil {
		_ = ra.server.Close()
		ra.server, ra.bound = nil, ""
	}
}

// StartRemote opens the panel to other devices if that was switched on, and keeps it open:
// routers forget forwarded ports and machines are handed new addresses.
// listenHost is the address the panel itself was started on.
func (a *App) StartRemote(ctx context.Context, handler http.Handler, listenHost string, port int) {
	ip := net.ParseIP(listenHost)
	a.remote.mu.Lock()
	a.remote.handler, a.remote.port = handler, port
	a.remote.fixed = listenHost != "localhost" && (ip == nil || !ip.IsLoopback())
	a.remote.mu.Unlock()

	refresh := func() {
		attempt, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		a.remote.apply(attempt, a.store.Setting("remote", remoteOff))
	}
	go func() {
		refresh()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if a.store.Setting("remote", remoteOff) != remoteOff {
					refresh()
				}
			}
		}
	}()
}
