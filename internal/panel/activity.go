package panel

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Activity is one thing that was done to a server, and by whom.
type Activity struct {
	ID   int64  `json:"id"`
	At   int64  `json:"at"`
	User string `json:"user"`
	Text string `json:"text"`
}

const activityKept = 500

// Log records an action against a server. Failures are ignored: the log must never block the action.
func (s *Store) Log(serverID, user, text string) {
	if _, err := s.db.Exec(`INSERT INTO activity (server_id, at, user, text) VALUES (?, ?, ?, ?)`, serverID, time.Now().Unix(), user, text); err != nil {
		return
	}
	_, _ = s.db.Exec(`DELETE FROM activity WHERE server_id = ? AND id NOT IN (SELECT id FROM activity WHERE server_id = ? ORDER BY id DESC LIMIT ?)`,
		serverID, serverID, activityKept)
}

func (s *Store) ActivityFor(serverID string, limit int) ([]Activity, error) {
	rows, err := s.db.Query(`SELECT id, at, user, text FROM activity WHERE server_id = ? ORDER BY id DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Activity{}
	for rows.Next() {
		var entry Activity
		if err := rows.Scan(&entry.ID, &entry.At, &entry.User, &entry.Text); err != nil {
			return nil, err
		}
		list = append(list, entry)
	}
	return list, rows.Err()
}

// log records an action by whoever made the request.
func (a *App) log(r *http.Request, serverID, text string) {
	user, _ := r.Context().Value(userKey).(User)
	name := user.Username
	if name == "" {
		name = "unknown"
	}
	a.store.Log(serverID, name, text)
}

func (a *App) handleActivity(w http.ResponseWriter, r *http.Request) {
	row, _, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > activityKept {
		limit = 100
	}
	list, err := a.store.ActivityFor(row.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// describePass turns a file or backup request that went through to the node into a log line.
// Reads return "" and are not logged.
func describePass(method, rest, query string) string {
	path := query
	if i := strings.Index(query, "path="); i >= 0 {
		path = query[i+5:]
		if j := strings.Index(path, "&"); j >= 0 {
			path = path[:j]
		}
		if decoded, err := queryUnescape(path); err == nil {
			path = decoded
		}
	}
	switch {
	case method == http.MethodPut && rest == "files/content":
		return "Saved " + path
	case method == http.MethodDelete && rest == "files":
		return "Deleted " + path
	case method == http.MethodPost && rest == "files/mkdir":
		return "Created a folder"
	case method == http.MethodPost && rest == "files/rename":
		return "Renamed a file"
	case method == http.MethodPost && rest == "backups":
		return "Took a backup"
	case method == http.MethodPost && strings.HasSuffix(rest, "/restore"):
		return "Restored a backup"
	case method == http.MethodDelete && strings.HasPrefix(rest, "backups/"):
		return "Deleted a backup"
	}
	return ""
}
