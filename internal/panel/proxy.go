package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// slowClient has no overall timeout: uploads, downloads and backups can take minutes.
var slowClient = &http.Client{}

// raw sends one request to a node's daemon and hands back the response for streaming.
func (n Node) raw(ctx context.Context, method, path, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(n.URL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+n.Token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return slowClient.Do(req)
}

// putFile writes a small file into a server's folder.
func (n Node) putFile(ctx context.Context, serverID, path string, content []byte) error {
	res, err := n.raw(ctx, http.MethodPut, "/servers/"+serverID+"/files/content?path="+queryEscape(path), "application/octet-stream", bytes.NewReader(content))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return daemonError(res)
}

// passed is what the browser may reach on the daemon through the panel.
// files/fetch is left out on purpose: only the panel decides what a node downloads.
var passed = []string{"files", "files/content", "files/mkdir", "files/rename", "backups", "console/history"}

func allowedPass(rest string) bool {
	for _, prefix := range passed {
		if rest == prefix {
			return true
		}
	}
	return strings.HasPrefix(rest, "backups/")
}

// handleNodeProxy forwards file and backup requests for one server to its node, streaming both ways.
func (a *App) handleNodeProxy(w http.ResponseWriter, r *http.Request) {
	rest := r.PathValue("rest")
	if !allowedPass(rest) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !a.mayUse(w, r, r.PathValue("id"), passPermission(rest)) {
		return
	}
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	path := "/servers/" + row.ID + "/" + rest
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	var body io.Reader
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		body = r.Body
	}
	res, err := node.raw(r.Context(), r.Method, path, r.Header.Get("Content-Type"), body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach the node")
		return
	}
	defer res.Body.Close()
	// A backup made by hand is copied off-site too, if the server asks for that.
	if r.Method == http.MethodPost && rest == "backups" && res.StatusCode < 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		var info struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(data, &info)
		a.copyOffsiteLater(node, row, info.Name)
		res.Body = io.NopCloser(bytes.NewReader(data))
	}
	for _, header := range []string{"Content-Type", "Content-Disposition", "Content-Length", "Last-Modified"} {
		if value := res.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}
	if res.StatusCode < 300 {
		if line := describePass(r.Method, rest, r.URL.RawQuery); line != "" {
			a.log(r, row.ID, line)
		}
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}
