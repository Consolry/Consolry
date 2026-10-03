package panel

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// releasesURL lists Consolry's published versions, newest first, pre-releases included.
var releasesURL = "https://api.github.com/repos/Consolry/Consolry/releases?per_page=1"

// UpdateStatus is what the panel knows about a newer version of itself.
type UpdateStatus struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	// Notes is the page describing the newest version.
	Notes string `json:"notes"`
	// Problem says why the check or the update did not work.
	Problem string `json:"problem"`
}

type release struct {
	Tag    string `json:"tag_name"`
	Page   string `json:"html_url"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// updater finds and installs newer versions of Consolry.
type updater struct {
	mu      sync.Mutex
	checked time.Time
	found   release
	problem string
	// restart closes Consolry and starts the program again. The command sets it.
	restart func()
}

var updateClient = &http.Client{Timeout: 5 * time.Minute}

// versionNumbers turns "v0.12.3-preview" into [0 12 3].
func versionNumbers(version string) []int {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if cut := strings.IndexAny(version, "-+"); cut >= 0 {
		version = version[:cut]
	}
	numbers := []int{}
	for _, part := range strings.Split(version, ".") {
		number, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		numbers = append(numbers, number)
	}
	return numbers
}

// newerVersion reports whether latest is a later version than current.
// A build without a real version number, such as one made from source, is never updated.
func newerVersion(latest, current string) bool {
	a, b := versionNumbers(latest), versionNumbers(current)
	if a == nil || b == nil {
		return false
	}
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// assetName is the release file for the machine this copy runs on.
func assetName() string {
	if runtime.GOOS == "windows" {
		return "Consolry.exe"
	}
	return "consolry-" + runtime.GOOS + "-" + runtime.GOARCH
}

// check asks GitHub for the newest version, at most once an hour unless fresh is set.
func (u *updater) check(ctx context.Context, current string, fresh bool) UpdateStatus {
	u.mu.Lock()
	defer u.mu.Unlock()
	if fresh || time.Since(u.checked) > time.Hour {
		u.checked = time.Now()
		found, err := latestRelease(ctx)
		if err != nil {
			u.problem = "Could not check for updates: " + err.Error()
		} else {
			u.found, u.problem = found, ""
		}
	}
	return UpdateStatus{
		Current:   current,
		Latest:    strings.TrimPrefix(u.found.Tag, "v"),
		Available: newerVersion(u.found.Tag, current),
		Notes:     u.found.Page,
		Problem:   u.problem,
	}
}

func latestRelease(ctx context.Context) (release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := updateClient.Do(req)
	if err != nil {
		return release{}, errors.New("GitHub could not be reached")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("GitHub answered %d", res.StatusCode)
	}
	var list []release
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&list); err != nil || len(list) == 0 {
		return release{}, errors.New("no published version was found")
	}
	return list[0], nil
}

func download(ctx context.Context, url string, into io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := updateClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("the download answered %d", res.StatusCode)
	}
	_, err = io.Copy(into, io.LimitReader(res.Body, 200<<20))
	return err
}

// install downloads the newest version, checks it against the published fingerprint and
// puts it in place of the running program. The old program is kept beside it as ".old"
// until the new one has started.
func (u *updater) install(ctx context.Context, current string) error {
	u.mu.Lock()
	found := u.found
	u.mu.Unlock()
	if !newerVersion(found.Tag, current) {
		return errors.New("there is no newer version to install")
	}
	var fileURL, sumsURL string
	for _, asset := range found.Assets {
		switch asset.Name {
		case assetName():
			fileURL = asset.URL
		case "checksums.txt":
			sumsURL = asset.URL
		}
	}
	if fileURL == "" || sumsURL == "" {
		return fmt.Errorf("version %s has no download for this kind of machine", found.Tag)
	}

	var sums strings.Builder
	if err := download(ctx, sumsURL, &sums); err != nil {
		return errors.New("could not download the list of fingerprints: " + err.Error())
	}
	expected := ""
	scanner := bufio.NewScanner(strings.NewReader(sums.String()))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == assetName() {
			expected = strings.ToLower(fields[0])
		}
	}
	if expected == "" {
		return errors.New("the new version has no published fingerprint, so it was not installed")
	}

	program, err := os.Executable()
	if err != nil {
		return err
	}
	fresh := program + ".new"
	file, err := os.OpenFile(fresh, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return errors.New("could not write next to the program: " + err.Error())
	}
	hash := sha256.New()
	err = download(ctx, fileURL, io.MultiWriter(file, hash))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil && hex.EncodeToString(hash.Sum(nil)) != expected {
		err = errors.New("the download did not match its published fingerprint, so it was not installed")
	}
	if err != nil {
		_ = os.Remove(fresh)
		return err
	}

	// A running program cannot be overwritten on Windows, but it can be renamed out of the way.
	old := program + ".old"
	_ = os.Remove(old)
	if err := os.Rename(program, old); err != nil {
		_ = os.Remove(fresh)
		return errors.New("could not replace the program: " + err.Error())
	}
	if err := os.Rename(fresh, program); err != nil {
		_ = os.Rename(old, program)
		return errors.New("could not replace the program: " + err.Error())
	}
	return nil
}

// OnRestart sets how Consolry restarts itself after installing an update.
func (a *App) OnRestart(restart func()) { a.updates.restart = restart }

func (a *App) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, a.updates.check(ctx, a.version, r.URL.Query().Get("fresh") != ""))
}

func (a *App) handleInstallUpdate(w http.ResponseWriter, r *http.Request) {
	if a.updates.restart == nil {
		writeError(w, http.StatusConflict, "this copy of Consolry cannot restart itself. Download the new version from the website instead")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	a.updates.check(ctx, a.version, true)
	if err := a.updates.install(ctx, a.version); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
	// Let the answer reach the browser before the panel goes away.
	go func() {
		time.Sleep(500 * time.Millisecond)
		a.updates.restart()
	}()
}
