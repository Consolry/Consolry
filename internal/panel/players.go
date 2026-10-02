package panel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// playerName covers Java names and Bedrock names joined through Geyser, which start with a dot.
// It also keeps anything typed here from smuggling a second console command.
var playerName = regexp.MustCompile(`^[A-Za-z0-9_.*]{1,32}$`)

var listReply = regexp.MustCompile(`There are (\d+) of a max(?: of)? (\d+) players online:?(.*)$`)

type bannedPlayer struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type playersView struct {
	Running          bool           `json:"running"`
	Online           []string       `json:"online"`
	Max              int            `json:"max"`
	Whitelist        []string       `json:"whitelist"`
	WhitelistEnabled bool           `json:"whitelistEnabled"`
	Operators        []string       `json:"operators"`
	Banned           []bannedPlayer `json:"banned"`
}

// readServerFile fetches one file from a server's folder, or nil if it does not exist.
func readServerFile(ctx context.Context, node Node, id, path string) []byte {
	res, err := node.raw(ctx, http.MethodGet, "/servers/"+id+"/files/content?path="+queryEscape(path), "", nil)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return data
}

func namesIn(data []byte) []string {
	var entries []struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(data, &entries)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name != "" {
			names = append(names, entry.Name)
		}
	}
	return names
}

// property reads one key from server.properties text.
func property(data []byte, key string) string {
	for _, line := range strings.Split(string(data), "\n") {
		if name, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok && name == key {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// onlinePlayers types "list" into the console and reads the answer out of the log.
func onlinePlayers(ctx context.Context, node Node, id string) ([]string, int) {
	var before []string
	if node.call(ctx, http.MethodGet, "/servers/"+id+"/console/history", nil, &before) != nil {
		return nil, 0
	}
	if node.call(ctx, http.MethodPost, "/servers/"+id+"/command", map[string]string{"command": "list"}, nil) != nil {
		return nil, 0
	}
	last := ""
	if len(before) > 0 {
		last = before[len(before)-1]
	}
	for attempt := 0; attempt < 10; attempt++ {
		time.Sleep(150 * time.Millisecond)
		var lines []string
		if node.call(ctx, http.MethodGet, "/servers/"+id+"/console/history", nil, &lines) != nil {
			return nil, 0
		}
		// Only look at lines printed after the command was sent.
		for i := len(lines) - 1; i >= 0 && lines[i] != last; i-- {
			match := listReply.FindStringSubmatch(lines[i])
			if match == nil {
				continue
			}
			limit, _ := strconv.Atoi(match[2])
			names := []string{}
			for _, name := range strings.Split(match[3], ",") {
				if name = strings.TrimSpace(name); name != "" {
					names = append(names, name)
				}
			}
			return names, limit
		}
	}
	return nil, 0
}

func (a *App) handlePlayers(w http.ResponseWriter, r *http.Request) {
	row, node, _, ok := a.minecraftServer(w, r)
	if !ok {
		return
	}
	properties := readServerFile(r.Context(), node, row.ID, "server.properties")
	view := playersView{
		Online:           []string{},
		Whitelist:        namesIn(readServerFile(r.Context(), node, row.ID, "whitelist.json")),
		WhitelistEnabled: property(properties, "white-list") == "true",
		Operators:        namesIn(readServerFile(r.Context(), node, row.ID, "ops.json")),
		Banned:           []bannedPlayer{},
	}
	view.Max, _ = strconv.Atoi(property(properties, "max-players"))
	_ = json.Unmarshal(readServerFile(r.Context(), node, row.ID, "banned-players.json"), &view.Banned)
	if view.Banned == nil {
		view.Banned = []bannedPlayer{}
	}

	if spec, err := serverState(r.Context(), node, row.ID); err == nil && spec.State == "running" {
		view.Running = true
		if names, limit := onlinePlayers(r.Context(), node, row.ID); names != nil {
			view.Online = names
			if limit > 0 {
				view.Max = limit
			}
		}
	}
	writeJSON(w, http.StatusOK, view)
}

// playerCommands maps a panel action to the console command that performs it.
var playerCommands = map[string]string{
	"whitelist-add":    "whitelist add %s",
	"whitelist-remove": "whitelist remove %s",
	"op":               "op %s",
	"deop":             "deop %s",
	"kick":             "kick %s",
	"ban":              "ban %s",
	"pardon":           "pardon %s",
}

func (a *App) handlePlayerAction(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Action string `json:"action"`
		Name   string `json:"name"`
		Reason string `json:"reason"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	row, node, _, ok := a.minecraftServer(w, r)
	if !ok {
		return
	}

	var command string
	switch input.Action {
	case "whitelist-on":
		command = "whitelist on"
	case "whitelist-off":
		command = "whitelist off"
	default:
		template, known := playerCommands[input.Action]
		if !known {
			writeError(w, http.StatusBadRequest, "unknown action")
			return
		}
		if !playerName.MatchString(input.Name) {
			writeError(w, http.StatusBadRequest, "that is not a valid player name")
			return
		}
		command = strings.Replace(template, "%s", input.Name, 1)
		if input.Action == "kick" || input.Action == "ban" {
			reason := strings.Join(strings.Fields(input.Reason), " ") // collapses any line breaks
			if len(reason) > 120 {
				reason = reason[:120]
			}
			if reason != "" {
				command += " " + reason
			}
		}
	}

	if err := node.call(r.Context(), http.MethodPost, "/servers/"+row.ID+"/command", map[string]string{"command": command}, nil); err != nil {
		writeError(w, http.StatusConflict, "start the server to manage players: "+err.Error())
		return
	}
	a.log(r, row.ID, "Ran the player command: "+command)
	// Give the server a moment to write its lists before the page reloads them.
	time.Sleep(400 * time.Millisecond)
	w.WriteHeader(http.StatusNoContent)
}
