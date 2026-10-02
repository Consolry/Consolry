package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var daemonClient = &http.Client{Timeout: 10 * time.Second}

// daemonSpec mirrors the server spec the daemon stores.
type daemonSpec struct {
	ID          string   `json:"id"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	StopCommand string   `json:"stopCommand"`
	State       string   `json:"state,omitempty"`
	StartedAt   int64    `json:"startedAt,omitempty"`
}

// call makes one quick request to a node's daemon and decodes the JSON reply into out.
func (n Node) call(ctx context.Context, method, path string, body, out any) error {
	return n.callWith(daemonClient, ctx, method, path, body, out)
}

// slow is call without a time limit, for downloads and anything that reads whole files.
func (n Node) slow(ctx context.Context, method, path string, body, out any) error {
	return n.callWith(slowClient, ctx, method, path, body, out)
}

func (n Node) callWith(client *http.Client, ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(n.URL, "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+n.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach the node: %w", err)
	}
	defer res.Body.Close()

	if err := daemonError(res); err != nil {
		return err
	}
	if out == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// daemonError turns a failed daemon reply into an error carrying the daemon's own message.
func daemonError(res *http.Response) error {
	if res.StatusCode < 400 {
		return nil
	}
	var failure struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(res.Body).Decode(&failure)
	if failure.Error == "" {
		failure.Error = res.Status
	}
	return errors.New(failure.Error)
}

// consoleURL is the daemon's WebSocket address for a server's console.
func (n Node) consoleURL(serverID string) string {
	base := strings.TrimRight(n.URL, "/")
	base = strings.Replace(base, "http", "ws", 1)
	return base + "/servers/" + serverID + "/console"
}

// splitCommand turns a typed command line into a program and its arguments,
// keeping quoted parts such as "C:\Program Files\Java\bin\java.exe" together.
func splitCommand(line string) (string, []string) {
	var parts []string
	var current strings.Builder
	var quote rune
	has := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, has = r, true
		case r == ' ' || r == '\t':
			if has {
				parts = append(parts, current.String())
				current.Reset()
				has = false
			}
		default:
			current.WriteRune(r)
			has = true
		}
	}
	if has {
		parts = append(parts, current.String())
	}
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}
