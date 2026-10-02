package daemon

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestReachable(t *testing.T) {
	for address, want := range map[string]bool{
		"203.0.113.7":    true,
		"8.8.8.8":        true,
		"192.168.1.1":    false, // the router is itself behind another router
		"10.0.0.5":       false,
		"100.72.14.9":    false, // shared by the internet provider between customers
		"100.63.255.1":   true,  // just outside the shared block
		"":               false,
		"not an address": false,
	} {
		if got := reachable(address); got != want {
			t.Errorf("reachable(%q) = %v, want %v", address, got, want)
		}
	}
}

// TestForwardPort talks to the real router on this network, so it only runs when asked:
// CONSOLRY_NETWORK_TESTS=1 go test ./internal/daemon/ -run TestForwardPort -v
func TestForwardPort(t *testing.T) {
	if os.Getenv("CONSOLRY_NETWORK_TESTS") == "" {
		t.Skip("set CONSOLRY_NETWORK_TESTS=1 to run tests that change real router settings")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const port = 25599
	result, err := forwardPort(ctx, port)
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	t.Logf("forwarded %d to %s; router's public address is %q (reachable from outside: %v)", port, result.InternalIP, result.ExternalIP, result.Reachable)
	if err := unforwardPort(ctx, port); err != nil {
		t.Fatalf("removing the forward again: %v", err)
	}
	t.Log("forward removed again")
}
