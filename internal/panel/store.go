// Package panel is the Consolry web panel: accounts, nodes, servers and the web interface.
package panel

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS nodes (
	id    INTEGER PRIMARY KEY,
	name  TEXT NOT NULL,
	url   TEXT NOT NULL,
	token TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS activity (
	id        INTEGER PRIMARY KEY,
	server_id TEXT NOT NULL,
	at        INTEGER NOT NULL,
	user      TEXT NOT NULL,
	text      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS activity_server ON activity (server_id, id);
CREATE TABLE IF NOT EXISTS schedules (
	id          INTEGER PRIMARY KEY,
	server_id   TEXT NOT NULL,
	action      TEXT NOT NULL,
	command     TEXT NOT NULL DEFAULT '',
	mode        TEXT NOT NULL,
	minutes     INTEGER NOT NULL DEFAULT 0,
	at          TEXT NOT NULL DEFAULT '',
	weekday     INTEGER NOT NULL DEFAULT 0,
	enabled     INTEGER NOT NULL DEFAULT 1,
	created_at  INTEGER NOT NULL,
	last_run    INTEGER NOT NULL DEFAULT 0,
	last_result TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS servers (
	id      TEXT PRIMARY KEY,
	name    TEXT NOT NULL,
	node_id INTEGER NOT NULL REFERENCES nodes(id)
);
CREATE TABLE IF NOT EXISTS server_users (
	server_id   TEXT NOT NULL,
	user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	permissions TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (server_id, user_id)
);
CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

const sessionLifetime = 30 * 24 * time.Hour

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	// Admin is the account that set the panel up. It manages the panel itself and can open every server.
	Admin bool `json:"admin"`
	// ServerLimit is how many servers of their own this account may create. The admin has no limit.
	ServerLimit int `json:"serverLimit"`
	// MemoryLimitMB is the most memory this account may give one of its servers.
	MemoryLimitMB int `json:"memoryLimitMb"`
}

type Node struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"-"`
}

type ServerRow struct {
	ID        string
	Name      string
	NodeID    int64
	OwnerID   int64  // the account that created the server
	Kind      string // "generic" or "minecraft"
	Software  string
	MCVersion string
}

type Store struct{ db *sql.DB }

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	// Columns added after the first version. SQLite has no "add if missing",
	// so a "duplicate column" error just means an earlier run already did this.
	for _, column := range []string{
		"servers ADD COLUMN kind TEXT NOT NULL DEFAULT 'generic'",
		"servers ADD COLUMN software TEXT NOT NULL DEFAULT ''",
		"servers ADD COLUMN mc_version TEXT NOT NULL DEFAULT ''",
		"servers ADD COLUMN forward TEXT NOT NULL DEFAULT ''",
		"servers ADD COLUMN owner_id INTEGER NOT NULL DEFAULT 0",
		"users ADD COLUMN admin INTEGER NOT NULL DEFAULT 0",
		"users ADD COLUMN server_limit INTEGER NOT NULL DEFAULT 0",
		"users ADD COLUMN memory_limit_mb INTEGER NOT NULL DEFAULT 4096",
	} {
		if _, err := db.Exec("ALTER TABLE " + column); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			return nil, err
		}
	}
	// Panels set up before there were several accounts: the first account is the admin.
	if _, err := db.Exec(`UPDATE users SET admin = 1 WHERE id = (SELECT MIN(id) FROM users) AND NOT EXISTS (SELECT 1 FROM users WHERE admin = 1)`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) HasUsers() (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count > 0, err
}

// CreateUser adds an account. The first one ever made becomes the admin.
func (s *Store) CreateUser(username, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	result, err := s.db.Exec(`INSERT INTO users (username, password_hash, admin) VALUES (?, ?, NOT EXISTS (SELECT 1 FROM users))`, username, string(hash))
	if err != nil {
		return User{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}
	if servers, memory := s.newAccountLimits(); memory > 0 {
		if err := s.SetLimits(id, servers, memory); err != nil {
			return User{}, err
		}
	}
	return s.UserByID(id)
}

var errBadLogin = errors.New("wrong username or password")

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("consolry"), bcrypt.DefaultCost)

func (s *Store) CheckPassword(username, password string) (User, error) {
	var user User
	var hash string
	err := s.db.QueryRow(`SELECT id, username, admin, server_limit, memory_limit_mb, password_hash FROM users WHERE username = ? COLLATE NOCASE`, username).
		Scan(&user.ID, &user.Username, &user.Admin, &user.ServerLimit, &user.MemoryLimitMB, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		// Hash anyway so a wrong username takes as long as a wrong password.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return User{}, errBadLogin
	}
	if err != nil {
		return User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, errBadLogin
	}
	return user, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewSession returns a token for the browser. Only its hash is stored.
func (s *Store) NewSession(userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	_, err := s.db.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		hashToken(token), userID, time.Now().Add(sessionLifetime).Unix())
	return token, err
}

func (s *Store) SessionUser(token string) (User, error) {
	var user User
	err := s.db.QueryRow(`
		SELECT users.id, users.username, users.admin, users.server_limit, users.memory_limit_mb FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = ? AND sessions.expires_at > ?`,
		hashToken(token), time.Now().Unix()).Scan(&user.ID, &user.Username, &user.Admin, &user.ServerLimit, &user.MemoryLimitMB)
	return user, err
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

func (s *Store) Nodes() ([]Node, error) {
	rows, err := s.db.Query(`SELECT id, name, url, token FROM nodes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := []Node{}
	for rows.Next() {
		var node Node
		if err := rows.Scan(&node.ID, &node.Name, &node.URL, &node.Token); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

func (s *Store) Node(id int64) (Node, error) {
	var node Node
	err := s.db.QueryRow(`SELECT id, name, url, token FROM nodes WHERE id = ?`, id).
		Scan(&node.ID, &node.Name, &node.URL, &node.Token)
	return node, err
}

func (s *Store) CreateNode(name, url, token string) (Node, error) {
	result, err := s.db.Exec(`INSERT INTO nodes (name, url, token) VALUES (?, ?, ?)`, name, url, token)
	if err != nil {
		return Node{}, err
	}
	id, err := result.LastInsertId()
	return Node{ID: id, Name: name, URL: url, Token: token}, err
}

func (s *Store) DeleteNode(id int64) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM servers WHERE node_id = ?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return errors.New("remove this node's servers first")
	}
	_, err := s.db.Exec(`DELETE FROM nodes WHERE id = ?`, id)
	return err
}

func (s *Store) Servers() ([]ServerRow, error) {
	rows, err := s.db.Query(`SELECT id, name, node_id, owner_id, kind, software, mc_version FROM servers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	servers := []ServerRow{}
	for rows.Next() {
		var row ServerRow
		if err := rows.Scan(&row.ID, &row.Name, &row.NodeID, &row.OwnerID, &row.Kind, &row.Software, &row.MCVersion); err != nil {
			return nil, err
		}
		servers = append(servers, row)
	}
	return servers, rows.Err()
}

func (s *Store) Server(id string) (ServerRow, error) {
	var row ServerRow
	err := s.db.QueryRow(`SELECT id, name, node_id, owner_id, kind, software, mc_version FROM servers WHERE id = ?`, id).
		Scan(&row.ID, &row.Name, &row.NodeID, &row.OwnerID, &row.Kind, &row.Software, &row.MCVersion)
	return row, err
}

func (s *Store) CreateServer(row ServerRow) error {
	if row.Kind == "" {
		row.Kind = "generic"
	}
	_, err := s.db.Exec(`INSERT INTO servers (id, name, node_id, owner_id, kind, software, mc_version) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.Name, row.NodeID, row.OwnerID, row.Kind, row.Software, row.MCVersion)
	return err
}

func (s *Store) DeleteServer(id string) error {
	if _, err := s.db.Exec(`DELETE FROM schedules WHERE server_id = ?`, id); err != nil {
		return err
	}
	_, _ = s.db.Exec(`DELETE FROM activity WHERE server_id = ?`, id)
	_, _ = s.db.Exec(`DELETE FROM server_users WHERE server_id = ?`, id)
	_, err := s.db.Exec(`DELETE FROM servers WHERE id = ?`, id)
	return err
}
