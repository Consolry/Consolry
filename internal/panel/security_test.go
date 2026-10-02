package panel

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestTOTPMatchesTheStandard(t *testing.T) {
	// RFC 6238's test key "12345678901234567890", whose code at 59 seconds ends in 287082.
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if got := totpAt(secret, 59/30); got != "287082" {
		t.Fatalf("code = %s, want 287082", got)
	}
	now := time.Unix(59, 0)
	if !validTOTP(secret, "287 082", now) || !validTOTP(secret, totpAt(secret, 2), now) {
		t.Fatal("the current code and the next one should both be accepted")
	}
	if validTOTP(secret, totpAt(secret, 5), now) || validTOTP(secret, "28708", now) {
		t.Fatal("a code from minutes away, or a short one, should be refused")
	}
}

func TestPasswordsAndTwoFactor(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	panel := httptest.NewServer(New(store, "test").Handler())
	defer panel.Close()
	browser := func() client {
		jar, _ := cookiejar.New(nil)
		return client{t: t, base: panel.URL, http: &http.Client{Jar: jar}}
	}
	admin, guest, laptop := browser(), browser(), browser()
	admin.do("POST", "/api/setup", map[string]string{"username": "admin", "password": "correct horse battery"}, nil)
	var who User
	guest.do("POST", "/api/signup", map[string]string{"username": "guest", "password": "first long password"}, &who)
	laptop.do("POST", "/api/login", map[string]string{"username": "guest", "password": "first long password"}, nil)

	// Changing the password needs the old one, and signs out every other device.
	if code := guest.do("POST", "/api/account/password", map[string]string{"current": "wrong", "new": "second long password"}, nil); code != http.StatusBadRequest {
		t.Fatalf("wrong current password got %d, want 400", code)
	}
	if code := guest.do("POST", "/api/account/password", map[string]string{"current": "first long password", "new": "second long password"}, nil); code != http.StatusNoContent {
		t.Fatalf("change password got %d, want 204", code)
	}
	if code := guest.do("GET", "/api/servers", nil, nil); code != http.StatusOK {
		t.Errorf("the browser that changed the password got %d, want 200", code)
	}
	if code := laptop.do("GET", "/api/servers", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("another device after the change got %d, want 401", code)
	}

	// Switching two-factor on needs a code that proves the app has the secret.
	var setup struct {
		Secret string `json:"secret"`
		QR     string `json:"qr"`
	}
	guest.do("POST", "/api/account/two-factor/start", map[string]any{}, &setup)
	if setup.Secret == "" || len(setup.QR) < 100 {
		t.Fatal("setup did not return a secret and a QR code")
	}
	if code := guest.do("POST", "/api/account/two-factor", map[string]string{"secret": setup.Secret, "code": "000000"}, nil); code != http.StatusBadRequest {
		t.Fatalf("a wrong code got %d, want 400", code)
	}
	var enabled struct {
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	now := totpAt(setup.Secret, time.Now().Unix()/30)
	if code := guest.do("POST", "/api/account/two-factor", map[string]string{"secret": setup.Secret, "code": now}, &enabled); code != http.StatusOK || len(enabled.RecoveryCodes) != 8 {
		t.Fatalf("enable got %d with %d backup codes", code, len(enabled.RecoveryCodes))
	}

	// Now the password alone is not enough.
	signIn := func() (client, string) {
		c := browser()
		var step struct {
			TwoFactor bool   `json:"twoFactor"`
			Ticket    string `json:"ticket"`
		}
		c.do("POST", "/api/login", map[string]string{"username": "guest", "password": "second long password"}, &step)
		if !step.TwoFactor || step.Ticket == "" {
			t.Fatal("signing in did not ask for a code")
		}
		if code := c.do("GET", "/api/servers", nil, nil); code != http.StatusUnauthorized {
			t.Fatalf("after the password only, got %d, want 401", code)
		}
		return c, step.Ticket
	}
	c, ticket := signIn()
	if code := c.do("POST", "/api/login/code", map[string]string{"ticket": ticket, "code": "123456"}, nil); code != http.StatusUnauthorized {
		t.Fatalf("a wrong code got %d, want 401", code)
	}
	if code := c.do("POST", "/api/login/code", map[string]string{"ticket": ticket, "code": totpAt(setup.Secret, time.Now().Unix()/30)}, nil); code != http.StatusOK {
		t.Fatalf("the right code got %d, want 200", code)
	}
	if code := c.do("GET", "/api/servers", nil, nil); code != http.StatusOK {
		t.Fatalf("after the code, got %d, want 200", code)
	}

	// A backup code works once.
	c, ticket = signIn()
	if code := c.do("POST", "/api/login/code", map[string]string{"ticket": ticket, "code": enabled.RecoveryCodes[0]}, nil); code != http.StatusOK {
		t.Fatalf("a backup code got %d, want 200", code)
	}
	c, ticket = signIn()
	if code := c.do("POST", "/api/login/code", map[string]string{"ticket": ticket, "code": enabled.RecoveryCodes[0]}, nil); code != http.StatusUnauthorized {
		t.Fatalf("a used backup code got %d, want 401", code)
	}

	// The admin can let someone who is locked out back in.
	reset := "/api/accounts/" + itoa(who.ID) + "/reset"
	if code := guest.do("POST", reset, map[string]any{"password": "taken over!!", "clearTwoFactor": true}, nil); code != http.StatusForbidden {
		t.Fatalf("a non-admin resetting got %d, want 403", code)
	}
	if code := admin.do("POST", reset, map[string]any{"password": "third long password", "clearTwoFactor": true}, nil); code != http.StatusNoContent {
		t.Fatalf("admin reset got %d, want 204", code)
	}
	if code := guest.do("GET", "/api/servers", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("after a reset the old session got %d, want 401", code)
	}
	var back User
	if code := browser().do("POST", "/api/login", map[string]string{"username": "guest", "password": "third long password"}, &back); code != http.StatusOK || back.TwoFactor {
		t.Errorf("signing in after the reset got %d, twoFactor=%v", code, back.TwoFactor)
	}
}
