package panel

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/DinoNaedYT/Consolry/internal/minecraft"
)

//go:embed all:web/dist
var webFiles embed.FS

const sessionCookie = "consolry_session"

type contextKey int

const userKey contextKey = 0

type App struct {
	store   *Store
	version string
}

func New(store *Store, version string) *App { return &App{store: store, version: version} }

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/state", a.handleState)
	mux.HandleFunc("POST /api/setup", a.handleSetup)
	mux.HandleFunc("POST /api/login", a.handleLogin)
	mux.HandleFunc("POST /api/logout", a.handleLogout)

	mux.Handle("GET /api/nodes", a.authed(a.handleNodes))
	mux.Handle("POST /api/nodes", a.authed(a.handleCreateNode))
	mux.Handle("DELETE /api/nodes/{id}", a.authed(a.handleDeleteNode))

	mux.Handle("GET /api/servers", a.authed(a.handleServers))
	mux.Handle("POST /api/servers", a.authed(a.handleCreateServer))
	mux.Handle("DELETE /api/servers/{id}", a.authed(a.handleDeleteServer))
	mux.Handle("PATCH /api/servers/{id}", a.authed(a.handleUpdateServer))
	mux.Handle("POST /api/servers/{id}/minecraft/version", a.authed(a.handleSwitchVersion))
	mux.Handle("GET /api/servers/{id}/activity", a.authed(a.handleActivity))
	mux.Handle("GET /api/servers/{id}/startup", a.authed(a.handleStartup))
	mux.Handle("GET /api/servers/{id}/network", a.authed(a.handleNetwork))
	mux.Handle("GET /api/servers/{id}/players", a.authed(a.handlePlayers))
	mux.Handle("POST /api/servers/{id}/players", a.authed(a.handlePlayerAction))
	mux.Handle("GET /api/servers/{id}/schedules", a.authed(a.handleSchedules))
	mux.Handle("POST /api/servers/{id}/schedules", a.authed(a.handleCreateSchedule))
	mux.Handle("PATCH /api/schedules/{sid}", a.authed(a.handleUpdateSchedule))
	mux.Handle("DELETE /api/schedules/{sid}", a.authed(a.handleDeleteSchedule))
	mux.Handle("POST /api/schedules/{sid}/run", a.authed(a.handleRunSchedule))
	mux.Handle("POST /api/servers/{id}/power", a.authed(a.handlePower))
	mux.Handle("GET /api/servers/{id}/console", a.authed(a.handleConsole))
	mux.Handle("GET /api/servers/{id}/diagnosis", a.authed(a.handleDiagnosis))
	mux.Handle("/api/servers/{id}/node/{rest...}", a.authed(a.handleNodeProxy))

	mux.Handle("GET /api/minecraft/software", a.authed(a.handleMinecraftSoftware))
	mux.Handle("GET /api/minecraft/versions", a.authed(a.handleMinecraftVersions))
	mux.Handle("GET /api/servers/{id}/plugins", a.authed(a.handlePlugins))
	mux.Handle("GET /api/servers/{id}/plugins/search", a.authed(a.handlePluginSearch))
	mux.Handle("POST /api/servers/{id}/plugins/install", a.authed(a.handlePluginInstall))
	mux.Handle("POST /api/servers/{id}/plugins/update", a.authed(a.handlePluginUpdate))

	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	}))
	mux.Handle("/", a.web())
	return mux
}

// web serves the built interface, falling back to index.html so page links work on refresh.
func (a *App) web() http.Handler {
	dist, err := fs.Sub(webFiles, "web/dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" {
			if _, err := fs.Stat(dist, name); err != nil {
				r = r.Clone(r.Context())
				r.URL.Path = "/"
			}
		}
		files.ServeHTTP(w, r)
	})
}

// --- sessions ---

func (a *App) currentUser(r *http.Request) (User, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return User{}, false
	}
	user, err := a.store.SessionUser(cookie.Value)
	return user, err == nil
}

func (a *App) authed(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.currentUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "sign in first")
			return
		}
		// A request that changes something must come from the panel's own pages, not another site.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if origin := r.Header.Get("Origin"); origin != "" {
				if parsed, err := url.Parse(origin); err != nil || parsed.Host != r.Host {
					writeError(w, http.StatusForbidden, "request came from another site")
					return
				}
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	})
}

func (a *App) startSession(w http.ResponseWriter, r *http.Request, user User) error {
	token, err := a.store.NewSession(user.ID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(sessionLifetime),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var validUsername = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,32}$`)

func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	hasUsers, err := a.store.HasUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	state := map[string]any{"setupNeeded": !hasUsers, "version": a.version, "user": nil}
	if user, ok := a.currentUser(r); ok {
		state["user"] = user
	}
	writeJSON(w, http.StatusOK, state)
}

// setupLock stops two simultaneous first-run requests from both creating an admin.
var setupLock sync.Mutex

func (a *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !readJSON(w, r, &input) {
		return
	}
	setupLock.Lock()
	defer setupLock.Unlock()

	hasUsers, err := a.store.HasUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if hasUsers {
		writeError(w, http.StatusConflict, "setup has already been completed")
		return
	}
	if !validUsername.MatchString(input.Username) {
		writeError(w, http.StatusBadRequest, "username must be 3 to 32 letters, digits, dots, dashes or underscores")
		return
	}
	if len(input.Password) < 10 {
		writeError(w, http.StatusBadRequest, "password must be at least 10 characters")
		return
	}
	user, err := a.store.CreateUser(input.Username, input.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.startSession(w, r, user); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !readJSON(w, r, &input) {
		return
	}
	user, err := a.store.CheckPassword(input.Username, input.Password)
	if errors.Is(err, errBadLogin) {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.startSession(w, r, user); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = a.store.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true})
	w.WriteHeader(http.StatusNoContent)
}

// --- nodes ---

type nodeView struct {
	Node
	Online  bool   `json:"online"`
	OS      string `json:"os,omitempty"`
	Version string `json:"version,omitempty"`
}

type health struct {
	Version string `json:"version"`
	OS      string `json:"os"`
}

func (a *App) handleNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := a.store.Nodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	views := make([]nodeView, len(nodes))
	for i, node := range nodes {
		views[i] = nodeView{Node: node}
		var h health
		if node.call(r.Context(), http.MethodGet, "/health", nil, &h) == nil {
			views[i].Online, views[i].OS, views[i].Version = true, h.OS, h.Version
		}
	}
	writeJSON(w, http.StatusOK, views)
}

func (a *App) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	input.Name, input.URL, input.Token = strings.TrimSpace(input.Name), strings.TrimSpace(input.URL), strings.TrimSpace(input.Token)
	parsed, err := url.Parse(input.URL)
	if input.Name == "" || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		writeError(w, http.StatusBadRequest, "give the node a name and an address starting with http:// or https://")
		return
	}
	candidate := Node{Name: input.Name, URL: input.URL, Token: input.Token}
	var h health
	if err := candidate.call(r.Context(), http.MethodGet, "/health", nil, &h); err != nil {
		writeError(w, http.StatusBadGateway, "the daemon did not accept that address and token: "+err.Error())
		return
	}
	node, err := a.store.CreateNode(input.Name, input.URL, input.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, nodeView{Node: node, Online: true, OS: h.OS, Version: h.Version})
}

func (a *App) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	if err := a.store.DeleteNode(id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- servers ---

type serverView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	NodeID      int64    `json:"nodeId"`
	NodeName    string   `json:"nodeName"`
	State       string   `json:"state"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	StopCommand string   `json:"stopCommand"`
	StartedAt   int64    `json:"startedAt"`
	Kind        string   `json:"kind"`
	Software    string   `json:"software"`
	MCVersion   string   `json:"mcVersion"`
	CPU         float64  `json:"cpu"`
	Memory      uint64   `json:"memory"`
	MemoryLimit int      `json:"memoryLimitMb"`
	Progress    string   `json:"progress"`
}

func (a *App) handleServers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.Servers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	nodes, err := a.store.Nodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Ask each node once for the live state of its servers.
	names := map[int64]string{}
	live := map[string]daemonSpec{}
	for _, node := range nodes {
		names[node.ID] = node.Name
		var specs []daemonSpec
		if node.call(r.Context(), http.MethodGet, "/servers", nil, &specs) == nil {
			for _, spec := range specs {
				live[strconv.FormatInt(node.ID, 10)+"/"+spec.ID] = spec
			}
		}
	}

	views := make([]serverView, len(rows))
	for i, row := range rows {
		view := serverView{
			ID: row.ID, Name: row.Name, NodeID: row.NodeID, NodeName: names[row.NodeID], State: "unreachable", Args: []string{},
			Kind: row.Kind, Software: row.Software, MCVersion: row.MCVersion,
		}
		if spec, ok := live[strconv.FormatInt(row.NodeID, 10)+"/"+row.ID]; ok {
			view.State, view.Command, view.StopCommand, view.StartedAt = spec.State, spec.Command, spec.StopCommand, spec.StartedAt
			if spec.Args != nil {
				view.Args = spec.Args
			}
			view.CPU, view.Memory, view.MemoryLimit, view.Progress = spec.CPU, spec.Memory, memoryOf(spec.Args), spec.Progress
		}
		views[i] = view
	}
	writeJSON(w, http.StatusOK, views)
}

var slugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

func newServerID(name string) (string, error) {
	slug := strings.Trim(slugUnsafe.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(slug) > 32 {
		slug = strings.Trim(slug[:32], "-")
	}
	if slug == "" {
		slug = "server"
	}
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	return slug + "-" + hex.EncodeToString(suffix), nil
}

func (a *App) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string          `json:"name"`
		NodeID      int64           `json:"nodeId"`
		StartLine   string          `json:"startCommand"`
		StopCommand string          `json:"stopCommand"`
		Minecraft   *minecraftInput `json:"minecraft"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "give the server a name")
		return
	}
	node, err := a.store.Node(input.NodeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "choose a node for this server")
		return
	}
	id, err := newServerID(input.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	row := ServerRow{ID: id, Name: input.Name, NodeID: node.ID}
	var spec daemonSpec
	var download minecraft.Download
	if input.Minecraft != nil {
		spec, download, err = minecraftSpec(r.Context(), id, *input.Minecraft)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		row.Kind, row.Software, row.MCVersion = "minecraft", input.Minecraft.Software, input.Minecraft.Version
	} else {
		command, args := splitCommand(input.StartLine)
		if command == "" {
			writeError(w, http.StatusBadRequest, "a start command is required")
			return
		}
		spec = daemonSpec{ID: id, Command: command, Args: args, StopCommand: strings.TrimSpace(input.StopCommand)}
	}

	if err := node.call(r.Context(), http.MethodPost, "/servers", spec, nil); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// If anything after this fails, take the half-made server back off the node.
	undo := func() { _ = node.call(context.WithoutCancel(r.Context()), http.MethodDelete, "/servers/"+id, nil, nil) }

	warning := ""
	if input.Minecraft != nil {
		if err := installMinecraft(r.Context(), node, id, download); err != nil {
			undo()
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		warning = ensureJava(r.Context(), node, download.JavaMin)
	}
	if err := a.store.CreateServer(row); err != nil {
		undo()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.log(r, id, "Created the server")
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "warning": warning})
}

// serverNode looks up a server and the node it lives on, answering 404 itself if either is missing.
func (a *App) serverNode(w http.ResponseWriter, r *http.Request) (ServerRow, Node, bool) {
	row, err := a.store.Server(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return ServerRow{}, Node{}, false
	}
	node, err := a.store.Node(row.NodeID)
	if err != nil {
		writeError(w, http.StatusNotFound, "this server's node no longer exists")
		return ServerRow{}, Node{}, false
	}
	return row, node, true
}

func (a *App) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	if err := node.call(r.Context(), http.MethodDelete, "/servers/"+row.ID, nil, nil); err != nil && err.Error() != "server not found" {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err := a.store.DeleteServer(row.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handlePower(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Action string `json:"action"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if input.Action != "start" && input.Action != "stop" && input.Action != "kill" {
		writeError(w, http.StatusBadRequest, "action must be start, stop or kill")
		return
	}
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	var result map[string]string
	if err := node.call(r.Context(), http.MethodPost, "/servers/"+row.ID+"/"+input.Action, nil, &result); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	a.log(r, row.ID, map[string]string{"start": "Started the server", "stop": "Stopped the server", "kill": "Killed the server"}[input.Action])
	writeJSON(w, http.StatusOK, result)
}

// handleConsole joins the browser's WebSocket to the daemon's, passing lines both ways.
func (a *App) handleConsole(w http.ResponseWriter, r *http.Request) {
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	upstream, _, err := websocket.Dial(ctx, node.consoleURL(row.ID), &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + node.Token}},
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach the node's console")
		return
	}
	defer upstream.CloseNow()
	upstream.SetReadLimit(2 << 20)

	// Accept only allows connections from the panel's own address, which blocks other sites.
	browser, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer browser.CloseNow()

	pipe := func(from, to *websocket.Conn) {
		defer cancel()
		for {
			kind, data, err := from.Read(ctx)
			if err != nil {
				return
			}
			if to.Write(ctx, kind, data) != nil {
				return
			}
		}
	}
	go pipe(browser, upstream)
	pipe(upstream, browser)
}

// --- helpers ---

func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	// Browsers cannot send JSON to another site without a preflight, so this also blocks cross-site form posts.
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "send JSON")
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
