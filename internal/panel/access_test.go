package panel

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/DinoNaedYT/Consolry/internal/daemon"
)

func TestSharedServerPermissions(t *testing.T) {
	const token = "test-token"
	manager, err := daemon.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown()
	node := httptest.NewServer(daemon.Handler(manager, token, "test"))
	defer node.Close()

	store, err := OpenStore(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	panel := httptest.NewServer(New(store, "test").Handler())
	defer panel.Close()

	signedIn := func() client {
		jar, _ := cookiejar.New(nil)
		return client{t: t, base: panel.URL, http: &http.Client{Jar: jar}}
	}
	admin, guest := signedIn(), signedIn()
	login := map[string]string{"username": "guest", "password": "a long guest password"}

	// Nobody can sign up before the panel has an admin.
	if code := guest.do("POST", "/api/signup", login, nil); code != http.StatusForbidden {
		t.Fatalf("sign-up before setup got %d, want 403", code)
	}
	if code := admin.do("POST", "/api/setup", map[string]string{"username": "admin", "password": "correct horse battery"}, nil); code != http.StatusCreated {
		t.Fatalf("setup got %d, want 201", code)
	}
	var who User
	if code := guest.do("POST", "/api/signup", login, &who); code != http.StatusCreated || who.Admin {
		t.Fatalf("sign-up got %d, admin=%v; want 201 and not an admin", code, who.Admin)
	}
	if code := signedIn().do("POST", "/api/signup", map[string]string{"username": "GUEST", "password": "another long password"}, nil); code != http.StatusConflict {
		t.Fatalf("taken username got %d, want 409", code)
	}

	var created struct {
		ID int64 `json:"id"`
	}
	admin.do("POST", "/api/nodes", map[string]string{"name": "local", "url": node.URL, "token": token}, &created)
	var server struct {
		ID string `json:"id"`
	}
	if code := admin.do("POST", "/api/servers", map[string]any{"name": "Shared", "nodeId": created.ID, "startCommand": "echo hi"}, &server); code != http.StatusCreated {
		t.Fatalf("create server got %d, want 201", code)
	}
	base := "/api/servers/" + server.ID

	// A new account manages nothing and sees nothing.
	for _, path := range []string{"/api/nodes", "/api/panel"} {
		if code := guest.do("GET", path, nil, nil); code != http.StatusForbidden {
			t.Errorf("guest GET %s got %d, want 403", path, code)
		}
	}
	if code := guest.do("POST", "/api/servers", map[string]any{"name": "Mine", "nodeId": created.ID, "startCommand": "echo"}, nil); code != http.StatusForbidden {
		t.Errorf("guest creating a server got %d, want 403", code)
	}
	var list []serverView
	guest.do("GET", "/api/servers", nil, &list)
	if len(list) != 0 {
		t.Fatalf("guest sees %d servers before being invited, want 0", len(list))
	}
	if code := guest.do("GET", base+"/activity", nil, nil); code != http.StatusNotFound {
		t.Errorf("guest on an unshared server got %d, want 404", code)
	}

	// Invite with two permissions; unknown ones are dropped.
	if code := admin.do("POST", base+"/users", map[string]any{"username": "nobody", "permissions": []string{"files"}}, nil); code != http.StatusNotFound {
		t.Errorf("inviting an unknown user got %d, want 404", code)
	}
	var member Member
	code := admin.do("POST", base+"/users", map[string]any{"username": "guest", "permissions": []string{"files", "activity", "root"}}, &member)
	if code != http.StatusOK || len(member.Permissions) != 2 {
		t.Fatalf("invite got %d with permissions %v", code, member.Permissions)
	}

	guest.do("GET", "/api/servers", nil, &list)
	if len(list) != 1 || list[0].Owner || len(list[0].Permissions) != 2 {
		t.Fatalf("guest's server list after the invite: %+v", list)
	}
	checks := []struct {
		method, path string
		want         int
	}{
		{"GET", base + "/activity", http.StatusOK},
		{"GET", base + "/node/files?path=", http.StatusOK},
		{"GET", base + "/node/backups", http.StatusForbidden},
		{"GET", base + "/node/console/history", http.StatusForbidden},
		{"GET", base + "/schedules", http.StatusForbidden},
		{"GET", base + "/users", http.StatusForbidden},
		{"DELETE", base, http.StatusForbidden},
	}
	for _, check := range checks {
		if code := guest.do(check.method, check.path, nil, nil); code != check.want {
			t.Errorf("guest %s %s got %d, want %d", check.method, check.path, code, check.want)
		}
	}
	if code := guest.do("POST", base+"/power", map[string]string{"action": "start"}, nil); code != http.StatusForbidden {
		t.Errorf("guest starting the server got %d, want 403", code)
	}

	// A schedule is guarded by its server's permissions too.
	var schedule Schedule
	if code := admin.do("POST", base+"/schedules", map[string]any{"action": "backup", "mode": "interval", "minutes": 60}, &schedule); code != http.StatusCreated {
		t.Fatalf("create schedule got %d, want 201", code)
	}
	path := "/api/schedules/" + itoa(schedule.ID)
	if code := guest.do("DELETE", path, nil, nil); code != http.StatusForbidden {
		t.Errorf("guest deleting a schedule got %d, want 403", code)
	}

	// Changing and removing access takes effect at once.
	if code := admin.do("PATCH", base+"/users/"+itoa(who.ID), map[string]any{"permissions": []string{"schedules"}}, nil); code != http.StatusNoContent {
		t.Fatalf("change permissions got %d, want 204", code)
	}
	if code := guest.do("GET", base+"/schedules", nil, nil); code != http.StatusOK {
		t.Errorf("guest with the schedules permission got %d, want 200", code)
	}
	if code := guest.do("GET", base+"/activity", nil, nil); code != http.StatusForbidden {
		t.Errorf("guest after losing a permission got %d, want 403", code)
	}
	if code := admin.do("DELETE", base+"/users/"+itoa(who.ID), nil, nil); code != http.StatusNoContent {
		t.Fatalf("remove member got %d, want 204", code)
	}
	if code := guest.do("GET", base+"/schedules", nil, nil); code != http.StatusNotFound {
		t.Errorf("guest after being removed got %d, want 404", code)
	}

	// Sign-up can be switched off.
	if code := admin.do("POST", "/api/panel", map[string]any{"signup": false}, nil); code != http.StatusOK {
		t.Fatalf("switching sign-up off got %d, want 200", code)
	}
	if code := signedIn().do("POST", "/api/signup", map[string]string{"username": "late", "password": "another long password"}, nil); code != http.StatusForbidden {
		t.Errorf("sign-up while switched off got %d, want 403", code)
	}
}

func itoa(value int64) string {
	digits := ""
	for ; value > 0; value /= 10 {
		digits = string(rune('0'+value%10)) + digits
	}
	return digits
}

func TestLimiter(t *testing.T) {
	l := newLimiter(2, 50*time.Millisecond)
	l.fail("a")
	if !l.allow("a") {
		t.Fatal("one failure should not lock an address out")
	}
	l.fail("a")
	if l.allow("a") {
		t.Fatal("two failures should lock the address out")
	}
	if !l.allow("b") {
		t.Fatal("another address should be unaffected")
	}
	time.Sleep(70 * time.Millisecond)
	if !l.allow("a") {
		t.Fatal("the lock should lift once the window has passed")
	}
}
