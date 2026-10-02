// Package panel is the Consolry web panel: accounts, nodes, servers and the web interface.
package panel

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
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
CREATE TABLE IF NOT EXISTS servers (
	id      TEXT PRIMARY KEY,
	name    TEXT NOT NULL,
	node_id INTEGER NOT NULL REFERENCES nodes(id)
);
`

const sessionLifetime = 30 * 24 * time.Hour

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Node struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"-"`
}

type ServerRow struct {
	ID     string
	Name   string
	NodeID int64
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
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) HasUsers() (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count > 0, err
}

func (s *Store) CreateUser(username, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	result, err := s.db.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, string(hash))
	if err != nil {
		return User{}, err
	}
	id, err := result.LastInsertId()
	return User{ID: id, Username: username}, err
}

var errBadLogin = errors.New("wrong username or password")

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("consolry"), bcrypt.DefaultCost)

func (s *Store) CheckPassword(username, password string) (User, error) {
	var user User
	var hash string
	err := s.db.QueryRow(`SELECT id, username, password_hash FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &hash)
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
		SELECT users.id, users.username FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = ? AND sessions.expires_at > ?`,
		hashToken(token), time.Now().Unix()).Scan(&user.ID, &user.Username)
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
	rows, err := s.db.Query(`SELECT id, name, node_id FROM servers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	servers := []ServerRow{}
	for rows.Next() {
		var row ServerRow
		if err := rows.Scan(&row.ID, &row.Name, &row.NodeID); err != nil {
			return nil, err
		}
		servers = append(servers, row)
	}
	return servers, rows.Err()
}

func (s *Store) Server(id string) (ServerRow, error) {
	var row ServerRow
	err := s.db.QueryRow(`SELECT id, name, node_id FROM servers WHERE id = ?`, id).Scan(&row.ID, &row.Name, &row.NodeID)
	return row, err
}

func (s *Store) CreateServer(row ServerRow) error {
	_, err := s.db.Exec(`INSERT INTO servers (id, name, node_id) VALUES (?, ?, ?)`, row.ID, row.Name, row.NodeID)
	return err
}

func (s *Store) DeleteServer(id string) error {
	_, err := s.db.Exec(`DELETE FROM servers WHERE id = ?`, id)
	return err
}
