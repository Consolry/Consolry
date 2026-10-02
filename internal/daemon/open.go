package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// LoadToken returns the shared secret the panel uses to talk to a daemon.
// CONSOLRY_DAEMON_TOKEN wins; otherwise one is generated once and kept in the data directory.
func LoadToken(dir string) (token string, created bool, err error) {
	if env := strings.TrimSpace(os.Getenv("CONSOLRY_DAEMON_TOKEN")); env != "" {
		return env, false, nil
	}
	path := filepath.Join(dir, "token")
	data, err := os.ReadFile(path)
	if err == nil {
		return strings.TrimSpace(string(data)), false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", false, err
	}
	token = hex.EncodeToString(raw)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, err
	}
	return token, true, os.WriteFile(path, []byte(token+"\n"), 0o600)
}
