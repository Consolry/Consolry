package panel

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
)

// --- one-time codes (RFC 6238), as shown by authenticator apps ---

var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32NoPad.EncodeToString(raw), nil
}

// totpAt is the six-digit code for one 30-second step.
func totpAt(secret string, step int64) string {
	key, err := base32NoPad.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1_000_000)
}

// validTOTP accepts the current code and the ones either side of it, for clocks that are a little off.
func validTOTP(secret, code string, now time.Time) bool {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != 6 || secret == "" {
		return false
	}
	step := now.Unix() / 30
	for _, candidate := range []int64{step - 1, step, step + 1} {
		if subtle.ConstantTimeCompare([]byte(totpAt(secret, candidate)), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// --- stored security details ---

func (s *Store) twoFactor(userID int64) (secret, recovery string) {
	_ = s.db.QueryRow(`SELECT totp_secret, recovery_codes FROM users WHERE id = ?`, userID).Scan(&secret, &recovery)
	return secret, recovery
}

func (s *Store) setTwoFactor(userID int64, secret string, recoveryHashes []string) error {
	_, err := s.db.Exec(`UPDATE users SET totp_secret = ?, recovery_codes = ? WHERE id = ?`, secret, strings.Join(recoveryHashes, ","), userID)
	return err
}

// useRecoveryCode signs in with one of the printed backup codes, which then stops working.
func (s *Store) useRecoveryCode(userID int64, code string) bool {
	_, stored := s.twoFactor(userID)
	wanted := hashToken(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), "-", "")))
	kept := []string{}
	found := false
	for _, hash := range strings.Split(stored, ",") {
		if hash == "" {
			continue
		}
		if !found && subtle.ConstantTimeCompare([]byte(hash), []byte(wanted)) == 1 {
			found = true
			continue
		}
		kept = append(kept, hash)
	}
	if found {
		_, _ = s.db.Exec(`UPDATE users SET recovery_codes = ? WHERE id = ?`, strings.Join(kept, ","), userID)
	}
	return found
}

func (s *Store) SetPassword(userID int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), userID)
	return err
}

// EndSessions signs an account out everywhere, except the session given, if any.
func (s *Store) EndSessions(userID int64, keepToken string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, hashToken(keepToken))
	return err
}

func newRecoveryCodes() (codes, hashes []string, err error) {
	const letters = "abcdefghjkmnpqrstuvwxyz23456789"
	for i := 0; i < 8; i++ {
		raw := make([]byte, 10)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}
		code := make([]byte, 10)
		for j, b := range raw {
			code[j] = letters[int(b)%len(letters)]
		}
		codes = append(codes, string(code[:5])+"-"+string(code[5:]))
		hashes = append(hashes, hashToken(string(code)))
	}
	return codes, hashes, nil
}

// --- signing in with a second step ---

// pendingLogin is a correct password still waiting for its one-time code.
type pendingLogin struct {
	userID  int64
	expires time.Time
}

var (
	pendingMu sync.Mutex
	pending   = map[string]pendingLogin{}
)

func newPendingLogin(userID int64) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)
	pendingMu.Lock()
	defer pendingMu.Unlock()
	for key, value := range pending {
		if time.Now().After(value.expires) {
			delete(pending, key)
		}
	}
	pending[ticket] = pendingLogin{userID: userID, expires: time.Now().Add(5 * time.Minute)}
	return ticket, nil
}

func (a *App) handleLoginCode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Ticket string `json:"ticket"`
		Code   string `json:"code"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	from := clientAddress(r)
	if !logins.allow(from) {
		writeError(w, http.StatusTooManyRequests, "too many wrong codes. Wait 15 minutes and try again")
		return
	}
	pendingMu.Lock()
	login, ok := pending[input.Ticket]
	pendingMu.Unlock()
	if !ok || time.Now().After(login.expires) {
		writeError(w, http.StatusUnauthorized, "that took too long. Sign in again")
		return
	}
	secret, _ := a.store.twoFactor(login.userID)
	if !validTOTP(secret, input.Code, time.Now()) && !a.store.useRecoveryCode(login.userID, input.Code) {
		logins.fail(from)
		writeError(w, http.StatusUnauthorized, "that code is not right. Check the time on your phone, or use one of your backup codes")
		return
	}
	pendingMu.Lock()
	delete(pending, input.Ticket)
	pendingMu.Unlock()
	logins.clear(from)
	user, err := a.store.UserByID(login.userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "that account no longer exists")
		return
	}
	if err := a.startSession(w, r, user); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// --- the signed-in person's own account ---

func sessionToken(r *http.Request) string {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		return cookie.Value
	}
	return ""
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	user, _ := r.Context().Value(userKey).(User)
	if _, err := a.store.CheckPassword(user.Username, input.Current); err != nil {
		writeError(w, http.StatusBadRequest, "your current password is not right")
		return
	}
	if len(input.New) < 10 {
		writeError(w, http.StatusBadRequest, "the new password must be at least 10 characters")
		return
	}
	if err := a.store.SetPassword(user.ID, input.New); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Anyone who knew the old password is signed out; this browser stays signed in.
	_ = a.store.EndSessions(user.ID, sessionToken(r))
	w.WriteHeader(http.StatusNoContent)
}

// handleStartTwoFactor makes a new secret for the authenticator app. It is only kept
// once the person proves their app has it, by sending back a code.
func (a *App) handleStartTwoFactor(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(User)
	secret, err := newTOTPSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	link := "otpauth://totp/" + url.PathEscape("Consolry:"+user.Username) + "?secret=" + secret + "&issuer=Consolry"
	png, err := qrcode.Encode(link, qrcode.Medium, 256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"secret": secret,
		"qr":     "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

func (a *App) handleEnableTwoFactor(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if !validTOTP(input.Secret, input.Code, time.Now()) {
		writeError(w, http.StatusBadRequest, "that code is not right. Check the time on your phone and try the newest code")
		return
	}
	user, _ := r.Context().Value(userKey).(User)
	codes, hashes, err := newRecoveryCodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := a.store.setTwoFactor(user.ID, strings.ToUpper(input.Secret), hashes); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"recoveryCodes": codes})
}

func (a *App) handleDisableTwoFactor(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password string `json:"password"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	user, _ := r.Context().Value(userKey).(User)
	if _, err := a.store.CheckPassword(user.Username, input.Password); err != nil {
		writeError(w, http.StatusBadRequest, "your password is not right")
		return
	}
	if err := a.store.setTwoFactor(user.ID, "", nil); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- the admin helping someone who is locked out ---

func (a *App) handleResetAccount(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password       string `json:"password"`
		ClearTwoFactor bool   `json:"clearTwoFactor"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	if _, err := a.store.UserByID(id); err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	if input.Password != "" {
		if len(input.Password) < 10 {
			writeError(w, http.StatusBadRequest, "the new password must be at least 10 characters")
			return
		}
		if err := a.store.SetPassword(id, input.Password); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if input.ClearTwoFactor {
		if err := a.store.setTwoFactor(id, "", nil); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	_ = a.store.EndSessions(id, "")
	w.WriteHeader(http.StatusNoContent)
}

// ResetAccount gives an account a new random password, switches off its two-factor login
// and signs it out everywhere. It is for the command line, so only someone at the
// machine itself can use it, such as an admin who forgot their own password.
func (s *Store) ResetAccount(username string) (string, error) {
	user, err := s.UserByName(username)
	if err != nil {
		return "", fmt.Errorf("there is no account called %q", username)
	}
	codes, _, err := newRecoveryCodes()
	if err != nil {
		return "", err
	}
	password := strings.Join(codes[:2], "-")
	if err := s.SetPassword(user.ID, password); err != nil {
		return "", err
	}
	if err := s.setTwoFactor(user.ID, "", nil); err != nil {
		return "", err
	}
	return password, s.EndSessions(user.ID, "")
}
