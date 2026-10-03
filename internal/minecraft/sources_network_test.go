package minecraft

import (
	"context"
	"os"
	"testing"
	"time"
)

// These talk to the real Hangar, so they only run when asked: CONSOLRY_NETWORK_TESTS=1.
func TestHangarForReal(t *testing.T) {
	if os.Getenv("CONSOLRY_NETWORK_TESTS") == "" {
		t.Skip("set CONSOLRY_NETWORK_TESTS=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	paper, _ := FindSoftware("paper")
	projects, total, err := HangarSearch(ctx, "viaversion", paper, "1.21.4", 0, 5)
	if err != nil || total == 0 || len(projects) == 0 {
		t.Fatalf("search: %d results, %v", total, err)
	}
	release, found, err := HangarLatest(ctx, "ViaVersion", "1.21.4")
	if err != nil || !found || release.SHA256 == "" || release.URL == "" {
		t.Fatalf("latest: %+v %v %v", release, found, err)
	}
	t.Logf("ViaVersion %s, %s", release.Version, release.Filename)
}
