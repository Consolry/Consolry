package panel

import (
	"errors"
	"net/http"
	"strconv"
)

// Account is one row of the admin's list of accounts.
type Account struct {
	User
	// Servers is how many servers this account owns.
	Servers int `json:"servers"`
}

// Limits on the memory an account may give one of its servers.
const (
	minMemoryMB     = 512
	maxMemoryMB     = 262144
	defaultMemoryMB = 4096
)

func (s *Store) Accounts() ([]Account, error) {
	rows, err := s.db.Query(`
		SELECT users.id, users.username, users.admin, users.server_limit, users.memory_limit_mb, users.totp_secret != '',
			(SELECT COUNT(*) FROM servers WHERE servers.owner_id = users.id)
		FROM users ORDER BY users.admin DESC, users.username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := []Account{}
	for rows.Next() {
		var account Account
		if err := rows.Scan(&account.ID, &account.Username, &account.Admin, &account.ServerLimit, &account.MemoryLimitMB, &account.TwoFactor, &account.Servers); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *Store) UserByID(id int64) (User, error) {
	var user User
	err := s.db.QueryRow(`SELECT id, username, admin, server_limit, memory_limit_mb, totp_secret != '' FROM users WHERE id = ?`, id).
		Scan(&user.ID, &user.Username, &user.Admin, &user.ServerLimit, &user.MemoryLimitMB, &user.TwoFactor)
	return user, err
}

func (s *Store) SetLimits(userID int64, servers, memoryMB int) error {
	_, err := s.db.Exec(`UPDATE users SET server_limit = ?, memory_limit_mb = ? WHERE id = ?`, servers, memoryMB, userID)
	return err
}

// OwnedServers counts the servers an account created.
func (s *Store) OwnedServers(userID int64) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM servers WHERE owner_id = ?`, userID).Scan(&count)
	return count, err
}

// DeleteUser removes an account. Servers it owned are kept and pass to the admin.
func (s *Store) DeleteUser(id int64) error {
	if _, err := s.db.Exec(`UPDATE servers SET owner_id = 0 WHERE owner_id = ?`, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM server_users WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ? AND admin = 0`, id)
	return err
}

// newAccountLimits is what an account gets when it signs up, as chosen by the admin.
func (s *Store) newAccountLimits() (servers, memoryMB int) {
	servers, _ = strconv.Atoi(s.Setting("new_server_limit", "0"))
	memoryMB, _ = strconv.Atoi(s.Setting("new_memory_limit_mb", strconv.Itoa(defaultMemoryMB)))
	return servers, memoryMB
}

func validLimits(servers, memoryMB int) error {
	if servers < 0 || servers > 100 {
		return errors.New("the number of servers must be between 0 and 100")
	}
	if memoryMB < minMemoryMB || memoryMB > maxMemoryMB {
		return errors.New("memory must be between 512 MB and 256 GB")
	}
	return nil
}

// memoryCap is the most memory a server may be given, or 0 for no limit.
// Servers made by the admin have none; anyone else's are held to their owner's allowance.
func (a *App) memoryCap(row ServerRow) int {
	owner, err := a.store.UserByID(row.OwnerID)
	if err != nil || owner.Admin {
		return 0
	}
	return owner.MemoryLimitMB
}

func (a *App) handleAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.store.Accounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	servers, memory := a.store.newAccountLimits()
	writeJSON(w, http.StatusOK, map[string]any{
		"accounts":    accounts,
		"newAccounts": map[string]int{"serverLimit": servers, "memoryLimitMb": memory},
	})
}

type limitsInput struct {
	ServerLimit   int `json:"serverLimit"`
	MemoryLimitMB int `json:"memoryLimitMb"`
}

func (a *App) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	var input limitsInput
	if !readJSON(w, r, &input) {
		return
	}
	if err := validLimits(input.ServerLimit, input.MemoryLimitMB); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	if _, err := a.store.UserByID(id); err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if err := a.store.SetLimits(id, input.ServerLimit, input.MemoryLimitMB); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleNewAccountLimits sets what accounts get when they sign up from now on.
func (a *App) handleNewAccountLimits(w http.ResponseWriter, r *http.Request) {
	var input limitsInput
	if !readJSON(w, r, &input) {
		return
	}
	if err := validLimits(input.ServerLimit, input.MemoryLimitMB); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.store.SetSetting("new_server_limit", strconv.Itoa(input.ServerLimit)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.store.SetSetting("new_memory_limit_mb", strconv.Itoa(input.MemoryLimitMB)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	account, err := a.store.UserByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if account.Admin {
		writeError(w, http.StatusConflict, "the admin account cannot be removed")
		return
	}
	if err := a.store.DeleteUser(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
