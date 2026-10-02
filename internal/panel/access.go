package panel

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Permissions are what a server's owner can hand to someone they invite.
// Everyone invited can see the server and its dashboard; each of these adds one part of it.
var Permissions = []string{"console", "power", "files", "plugins", "players", "settings", "backups", "schedules", "network", "activity"}

// ownerOnly marks a route that only the server's owner, or an admin, may use.
const ownerOnly = "owner"

func validPermission(name string) bool {
	for _, known := range Permissions {
		if known == name {
			return true
		}
	}
	return false
}

// cleanPermissions drops anything unknown or repeated and puts the rest in a fixed order.
func cleanPermissions(input []string) []string {
	chosen := map[string]bool{}
	for _, name := range input {
		if validPermission(name) {
			chosen[name] = true
		}
	}
	list := []string{}
	for _, name := range Permissions {
		if chosen[name] {
			list = append(list, name)
		}
	}
	return list
}

// Member is someone a server has been shared with.
type Member struct {
	UserID      int64    `json:"userId"`
	Username    string   `json:"username"`
	Permissions []string `json:"permissions"`
}

func splitPermissions(raw string) []string {
	if raw == "" {
		return []string{}
	}
	return cleanPermissions(strings.Split(raw, ","))
}

func (s *Store) Members(serverID string) ([]Member, error) {
	rows, err := s.db.Query(`
		SELECT users.id, users.username, server_users.permissions FROM server_users
		JOIN users ON users.id = server_users.user_id
		WHERE server_users.server_id = ? ORDER BY users.username`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []Member{}
	for rows.Next() {
		var member Member
		var raw string
		if err := rows.Scan(&member.UserID, &member.Username, &raw); err != nil {
			return nil, err
		}
		member.Permissions = splitPermissions(raw)
		members = append(members, member)
	}
	return members, rows.Err()
}

// SetMember shares a server with a user, or changes what they may do there.
func (s *Store) SetMember(serverID string, userID int64, permissions []string) error {
	_, err := s.db.Exec(`
		INSERT INTO server_users (server_id, user_id, permissions) VALUES (?, ?, ?)
		ON CONFLICT (server_id, user_id) DO UPDATE SET permissions = excluded.permissions`,
		serverID, userID, strings.Join(cleanPermissions(permissions), ","))
	return err
}

func (s *Store) RemoveMember(serverID string, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM server_users WHERE server_id = ? AND user_id = ?`, serverID, userID)
	return err
}

func (s *Store) UserByName(username string) (User, error) {
	var user User
	err := s.db.QueryRow(`SELECT id, username, admin FROM users WHERE username = ? COLLATE NOCASE`, username).
		Scan(&user.ID, &user.Username, &user.Admin)
	return user, err
}

// Setting reads one panel-wide setting, or the fallback if it has never been set.
func (s *Store) Setting(key, fallback string) string {
	var value string
	if s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value) != nil {
		return fallback
	}
	return value
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// access is what one user may do on one server.
type access struct {
	owner bool
	can   map[string]bool
}

func (g access) allows(permission string) bool {
	if g.owner || permission == "" {
		return true
	}
	return g.can[permission]
}

// list is the form sent to the browser, which uses it to decide which tabs to show.
func (g access) list() []string {
	if g.owner {
		return append([]string{}, Permissions...)
	}
	list := []string{}
	for name := range g.can {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

// accessTo works out what a user may do on a server. The second result is false
// when the server has not been shared with them at all.
func (s *Store) accessTo(user User, row ServerRow) (access, bool) {
	if user.Admin || row.OwnerID == user.ID {
		return access{owner: true}, true
	}
	var raw string
	if s.db.QueryRow(`SELECT permissions FROM server_users WHERE server_id = ? AND user_id = ?`, row.ID, user.ID).Scan(&raw) != nil {
		return access{}, false
	}
	granted := access{can: map[string]bool{}}
	for _, name := range splitPermissions(raw) {
		granted.can[name] = true
	}
	return granted, true
}

// onServer guards a route that acts on one server: the user must be signed in, the server
// must be shared with them, and they must hold the permission the route needs.
// A server that is not shared answers "not found", the same as one that does not exist.
func (a *App) onServer(permission string, next http.HandlerFunc) http.Handler {
	return a.authed(func(w http.ResponseWriter, r *http.Request) {
		if !a.mayUse(w, r, r.PathValue("id"), permission) {
			return
		}
		next(w, r)
	})
}

func (a *App) mayUse(w http.ResponseWriter, r *http.Request, serverID, permission string) bool {
	user, _ := r.Context().Value(userKey).(User)
	row, err := a.store.Server(serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return false
	}
	granted, shared := a.store.accessTo(user, row)
	if !shared {
		writeError(w, http.StatusNotFound, "server not found")
		return false
	}
	if permission == ownerOnly && !granted.owner || !granted.allows(permission) {
		writeError(w, http.StatusForbidden, "you do not have permission to do that on this server")
		return false
	}
	return true
}

// onSchedule guards the routes that name a schedule rather than a server.
func (a *App) onSchedule(next http.HandlerFunc) http.Handler {
	return a.authed(func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.PathValue("sid"), 10, 64)
		schedule, err := a.store.Schedule(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "schedule not found")
			return
		}
		if !a.mayUse(w, r, schedule.ServerID, "schedules") {
			return
		}
		next(w, r)
	})
}

// admin guards the routes that manage the panel itself.
func (a *App) admin(next http.HandlerFunc) http.Handler {
	return a.authed(func(w http.ResponseWriter, r *http.Request) {
		if user, _ := r.Context().Value(userKey).(User); !user.Admin {
			writeError(w, http.StatusForbidden, "only the panel's admin can do that")
			return
		}
		next(w, r)
	})
}

// passPermission is the permission a request passed straight to the node needs.
func passPermission(rest string) string {
	switch {
	case rest == "console/history":
		return "console"
	case rest == "backups" || strings.HasPrefix(rest, "backups/"):
		return "backups"
	}
	return "files"
}

// --- people on a server ---

func (a *App) handleMembers(w http.ResponseWriter, r *http.Request) {
	members, err := a.store.Members(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (a *App) handleInvite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username    string   `json:"username"`
		Permissions []string `json:"permissions"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	row, err := a.store.Server(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	invited, err := a.store.UserByName(strings.TrimSpace(input.Username))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "nobody has an account with that username. Ask them to create one on this panel's sign-up page first")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if invited.Admin || invited.ID == row.OwnerID {
		writeError(w, http.StatusConflict, invited.Username+" already has full access to this server")
		return
	}
	if err := a.store.SetMember(row.ID, invited.ID, input.Permissions); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.log(r, row.ID, "Shared the server with "+invited.Username)
	writeJSON(w, http.StatusOK, Member{UserID: invited.ID, Username: invited.Username, Permissions: cleanPermissions(input.Permissions)})
}

func (a *App) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Permissions []string `json:"permissions"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	member, ok := a.member(w, r)
	if !ok {
		return
	}
	if err := a.store.SetMember(r.PathValue("id"), member.UserID, input.Permissions); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.log(r, r.PathValue("id"), "Changed what "+member.Username+" can do")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	member, ok := a.member(w, r)
	if !ok {
		return
	}
	if err := a.store.RemoveMember(r.PathValue("id"), member.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.log(r, r.PathValue("id"), "Removed "+member.Username+" from the server")
	w.WriteHeader(http.StatusNoContent)
}

// member finds the person named in the request path among the server's members.
func (a *App) member(w http.ResponseWriter, r *http.Request) (Member, bool) {
	id, _ := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	members, _ := a.store.Members(r.PathValue("id"))
	for _, member := range members {
		if member.UserID == id {
			return member, true
		}
	}
	writeError(w, http.StatusNotFound, "that person is not on this server")
	return Member{}, false
}

// --- signing up ---

func (a *App) signupAllowed() bool { return a.store.Setting("signup", "on") == "on" }

func (a *App) handleSignup(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !readJSON(w, r, &input) {
		return
	}
	hasUsers, err := a.store.HasUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !hasUsers || !a.signupAllowed() {
		writeError(w, http.StatusForbidden, "this panel is not accepting new accounts")
		return
	}
	if !signups.allow(clientAddress(r)) {
		writeError(w, http.StatusTooManyRequests, "too many accounts were created from your connection. Try again later")
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
	if _, err := a.store.UserByName(input.Username); err == nil {
		writeError(w, http.StatusConflict, "that username is taken")
		return
	}
	signups.fail(clientAddress(r)) // each new account counts towards the limit
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

// --- slowing down guessing ---

// limiter counts recent events per address and refuses once there have been too many.
type limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	seen   map[string][]time.Time
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{limit: limit, window: window, seen: map[string][]time.Time{}}
}

func (l *limiter) recent(address string) []time.Time {
	cutoff := time.Now().Add(-l.window)
	kept := l.seen[address][:0]
	for _, at := range l.seen[address] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(l.seen, address)
	} else {
		l.seen[address] = kept
	}
	return kept
}

func (l *limiter) allow(address string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(address)) < l.limit
}

func (l *limiter) fail(address string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen[address] = append(l.recent(address), time.Now())
}

func (l *limiter) clear(address string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.seen, address)
}

var (
	// Eight wrong passwords from one address locks that address out for a quarter of an hour.
	logins = newLimiter(8, 15*time.Minute)
	// One address may create a handful of accounts a day.
	signups = newLimiter(5, 24*time.Hour)
)

// clientAddress is the address a request came from, without the port.
func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// fromThisMachine reports whether a request was made on the computer the panel runs on.
func fromThisMachine(r *http.Request) bool {
	ip := net.ParseIP(clientAddress(r))
	return ip != nil && ip.IsLoopback()
}

// --- panel settings ---

func (a *App) handlePanelSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"signup": a.signupAllowed(), "remote": a.remote.status()})
}

func (a *App) handleUpdatePanelSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Signup *bool   `json:"signup"`
		Remote *string `json:"remote"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if input.Signup != nil {
		value := "off"
		if *input.Signup {
			value = "on"
		}
		if err := a.store.SetSetting("signup", value); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if input.Remote != nil {
		if *input.Remote != remoteOff && *input.Remote != remoteNetwork && *input.Remote != remoteInternet {
			writeError(w, http.StatusBadRequest, "choose off, network or internet")
			return
		}
		if err := a.store.SetSetting("remote", *input.Remote); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		a.remote.apply(ctx, *input.Remote)
	}
	a.handlePanelSettings(w, r)
}
