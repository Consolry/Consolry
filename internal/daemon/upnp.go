package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway2"
)

// gateway is the part of a home router's UPnP service that port forwarding needs.
// Routers offer it under one of three service names, which all share these calls.
type gateway interface {
	AddPortMappingCtx(ctx context.Context, remoteHost string, externalPort uint16, protocol string, internalPort uint16, internalClient string, enabled bool, description string, leaseSeconds uint32) error
	DeletePortMappingCtx(ctx context.Context, remoteHost string, externalPort uint16, protocol string) error
	GetExternalIPAddressCtx(ctx context.Context) (string, error)
	LocalAddr() net.IP
}

var errNoGateway = errors.New("your router did not answer. UPnP may be switched off in its settings, or this machine may not be connected through a home router")

// findGateway looks for a router on the local network that will forward ports on request.
func findGateway(ctx context.Context) (gateway, error) {
	if clients, _, err := internetgateway2.NewWANIPConnection2ClientsCtx(ctx); err == nil && len(clients) > 0 {
		return clients[0], nil
	}
	if clients, _, err := internetgateway2.NewWANIPConnection1ClientsCtx(ctx); err == nil && len(clients) > 0 {
		return clients[0], nil
	}
	if clients, _, err := internetgateway2.NewWANPPPConnection1ClientsCtx(ctx); err == nil && len(clients) > 0 {
		return clients[0], nil
	}
	return nil, errNoGateway
}

// sharedRange is the address block internet providers use when many customers share one public address.
var sharedRange = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// reachable reports whether an address the router gave as "public" can actually be reached from
// the internet. A private or shared address means the provider sits another router in front.
func reachable(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsUnspecified() && !sharedRange.Contains(ip)
}

type forwardResult struct {
	Port       int    `json:"port"`
	InternalIP string `json:"internalIp"`
	ExternalIP string `json:"externalIp"`
	// Reachable is false when the router's own address is not a public one, so the
	// forward exists but people outside still cannot connect.
	Reachable bool `json:"reachable"`
}

// forwardPort asks the router to send a TCP port from the internet to this machine.
func forwardPort(ctx context.Context, port int) (forwardResult, error) {
	router, err := findGateway(ctx)
	if err != nil {
		return forwardResult{}, err
	}
	local := router.LocalAddr().String()
	// A lease of 0 means "until removed". Some routers insist on a time limit, so fall back to a week;
	// the panel asks again every time the server starts.
	err = router.AddPortMappingCtx(ctx, "", uint16(port), "TCP", uint16(port), local, true, "Consolry", 0)
	if err != nil {
		err = router.AddPortMappingCtx(ctx, "", uint16(port), "TCP", uint16(port), local, true, "Consolry", 7*24*3600)
	}
	if err != nil {
		return forwardResult{}, errors.New("your router refused to open the port: " + err.Error())
	}
	external, _ := router.GetExternalIPAddressCtx(ctx)
	return forwardResult{Port: port, InternalIP: local, ExternalIP: external, Reachable: reachable(external)}, nil
}

func unforwardPort(ctx context.Context, port int) error {
	router, err := findGateway(ctx)
	if err != nil {
		return err
	}
	return router.DeletePortMappingCtx(ctx, "", uint16(port), "TCP")
}

func registerForwarding(mux *http.ServeMux) {
	port := func(w http.ResponseWriter, raw string) (int, bool) {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1024 || value > 65535 {
			writeError(w, http.StatusBadRequest, "the port must be a number from 1024 to 65535")
			return 0, false
		}
		return value, true
	}

	mux.HandleFunc("POST /network/forward", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Port int `json:"port"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&input)
		value, ok := port(w, strconv.Itoa(input.Port))
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		result, err := forwardPort(ctx, value)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("DELETE /network/forward", func(w http.ResponseWriter, r *http.Request) {
		value, ok := port(w, r.URL.Query().Get("port"))
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if err := unforwardPort(ctx, value); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
