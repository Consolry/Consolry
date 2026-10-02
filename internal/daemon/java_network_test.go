package daemon

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestInstallJava downloads a real Java runtime, so it only runs when asked:
// CONSOLRY_NETWORK_TESTS=1 go test ./internal/daemon/ -run TestInstallJava
func TestInstallJava(t *testing.T) {
	if os.Getenv("CONSOLRY_NETWORK_TESTS") == "" {
		t.Skip("set CONSOLRY_NETWORK_TESTS=1 to run tests that download from the internet")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := installJava(ctx, dir, 21); err != nil {
		t.Fatal(err)
	}
	if got := managedVersions(dir); len(got) != 1 || got[0] != 21 {
		t.Fatalf("managed versions = %v, want [21]", got)
	}
	if info := javaAt(javaBinary(dir, 21)); !info.Found || info.Major != 21 {
		t.Fatalf("the installed Java reports %+v", info)
	}
}
