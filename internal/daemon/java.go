package daemon

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type javaInfo struct {
	Found   bool   `json:"found"`
	Version string `json:"version"`
	Major   int    `json:"major"`
}

var javaVersion = regexp.MustCompile(`version "([^"]+)"`)

// detectJava reports the Java on this machine's PATH.
func detectJava() javaInfo { return javaAt("java") }

func javaAt(program string) javaInfo {
	out, err := exec.Command(program, "-version").CombinedOutput()
	if err != nil {
		return javaInfo{}
	}
	match := javaVersion.FindSubmatch(out)
	if match == nil {
		return javaInfo{}
	}
	version := string(match[1])
	parts := strings.Split(version, ".")
	major, _ := strconv.Atoi(parts[0])
	if major == 1 && len(parts) > 1 { // Java 8 calls itself 1.8
		major, _ = strconv.Atoi(parts[1])
	}
	return javaInfo{Found: true, Version: version, Major: major}
}

func javaBinary(dir string, major int) string {
	name := "java"
	if runtime.GOOS == "windows" {
		name = "java.exe"
	}
	return filepath.Join(dir, strconv.Itoa(major), "bin", name)
}

// managedVersions lists the Java versions Consolry has installed for itself, oldest first.
func managedVersions(dir string) []int {
	entries, _ := os.ReadDir(dir)
	var majors []int
	for _, entry := range entries {
		major, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if _, err := os.Stat(javaBinary(dir, major)); err == nil {
			majors = append(majors, major)
		}
	}
	sort.Ints(majors)
	return majors
}

// javaFor picks the program to run for a server that needs at least the given Java version:
// the machine's own Java if it is new enough, otherwise one Consolry installed.
func javaFor(dir string, needed int) (string, error) {
	if system := detectJava(); system.Found && system.Major >= needed {
		return "java", nil
	}
	for _, major := range managedVersions(dir) {
		if major >= needed {
			return javaBinary(dir, major), nil
		}
	}
	return "", fmt.Errorf("Java %d or newer is not installed on this machine", needed)
}

var javaInstall sync.Mutex

// installJava downloads the Eclipse Temurin runtime for a Java version into the data directory.
// It never touches the machine's own Java.
func installJava(ctx context.Context, dir string, major int) error {
	javaInstall.Lock()
	defer javaInstall.Unlock()
	if _, err := os.Stat(javaBinary(dir, major)); err == nil {
		return nil
	}

	system := map[string]string{"windows": "windows", "linux": "linux", "darwin": "mac"}[runtime.GOOS]
	arch := map[string]string{"amd64": "x64", "arm64": "aarch64"}[runtime.GOARCH]
	if system == "" || arch == "" {
		return fmt.Errorf("automatic Java install is not available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var assets []struct {
		Binary struct {
			Package struct {
				Link     string `json:"link"`
				Checksum string `json:"checksum"`
				Name     string `json:"name"`
			} `json:"package"`
		} `json:"binary"`
	}
	lookup := fmt.Sprintf("https://api.adoptium.net/v3/assets/latest/%d/hotspot?architecture=%s&image_type=jre&os=%s&vendor=eclipse", major, arch, system)
	if err := fetchJSON(ctx, lookup, &assets); err != nil {
		return fmt.Errorf("could not look up Java %d: %w", major, err)
	}
	if len(assets) == 0 || assets[0].Binary.Package.Link == "" {
		return fmt.Errorf("no Java %d download exists for this machine", major)
	}
	pkg := assets[0].Binary.Package

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	archive := filepath.Join(dir, "download-"+strconv.Itoa(major)+".part")
	defer os.Remove(archive)
	if err := downloadTo(ctx, pkg.Link, archive, pkg.Checksum); err != nil {
		return fmt.Errorf("could not download Java %d: %w", major, err)
	}

	// Unpack beside the final location, then swap it in, so a failed install leaves nothing half-made.
	staging := filepath.Join(dir, "unpacking-"+strconv.Itoa(major))
	os.RemoveAll(staging)
	defer os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	var err error
	if strings.HasSuffix(pkg.Name, ".zip") {
		err = unzipStripped(archive, staging)
	} else {
		err = untarStripped(archive, staging)
	}
	if err != nil {
		return fmt.Errorf("could not unpack Java %d: %w", major, err)
	}
	final := filepath.Join(dir, strconv.Itoa(major))
	os.RemoveAll(final)
	if err := os.Rename(staging, final); err != nil {
		return err
	}
	if !javaAt(javaBinary(dir, major)).Found {
		os.RemoveAll(final)
		return fmt.Errorf("the downloaded Java %d does not run on this machine", major)
	}
	return nil
}

func fetchJSON(ctx context.Context, address string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := fetchClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errors.New("the server answered " + res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// downloadTo saves a URL to a file and checks its SHA-256 fingerprint.
func downloadTo(ctx context.Context, address, path, sha256hex string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := fetchClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errors.New("the server answered " + res.Status)
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	sum := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, sum), res.Body)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if sha256hex != "" && !strings.EqualFold(sha256hex, hex.EncodeToString(sum.Sum(nil))) {
		return errors.New("the download did not match its published fingerprint")
	}
	return nil
}

// stripTop removes an archive entry's leading folder, such as "jdk-21.0.4+7-jre/".
func stripTop(name string) string {
	name = strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./")
	if i := strings.Index(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return ""
}

func unzipStripped(archive, dest string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, item := range reader.File {
		rel := stripTop(item.Name)
		if rel == "" {
			continue
		}
		rel = filepath.FromSlash(strings.TrimSuffix(rel, "/"))
		if item.FileInfo().IsDir() {
			if err := root.MkdirAll(rel, 0o755); err != nil {
				return err
			}
			continue
		}
		src, err := item.Open()
		if err != nil {
			return err
		}
		err = writeFile(root, rel, src)
		src.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func untarStripped(archive, dest string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	reader := tar.NewReader(unzipped)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel := stripTop(header.Name)
		if rel == "" {
			continue
		}
		rel = filepath.FromSlash(strings.TrimSuffix(rel, "/"))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(rel, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(root, rel, reader); err != nil {
				return err
			}
			// Keep the executable bit, or the java program will not run.
			if err := root.Chmod(rel, os.FileMode(header.Mode)&0o777); err != nil {
				return err
			}
		case tar.TypeSymlink:
			_ = root.Symlink(header.Linkname, rel)
		}
	}
}

func registerJava(mux *http.ServeMux, m *Manager) {
	mux.HandleFunc("GET /java", func(w http.ResponseWriter, r *http.Request) {
		system := detectJava()
		managed := managedVersions(m.javaDir())
		if managed == nil {
			managed = []int{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"found": system.Found, "version": system.Version, "major": system.Major, "managed": managed,
		})
	})

	// Ensure a Java version new enough for a server exists, installing one if the machine has none.
	mux.HandleFunc("POST /java/{major}", func(w http.ResponseWriter, r *http.Request) {
		major, err := strconv.Atoi(r.PathValue("major"))
		if err != nil || major < 8 || major > 99 {
			writeError(w, http.StatusBadRequest, "unknown Java version")
			return
		}
		if _, err := javaFor(m.javaDir(), major); err == nil {
			writeJSON(w, http.StatusOK, map[string]bool{"installed": false})
			return
		}
		if err := installJava(r.Context(), m.javaDir(), major); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"installed": true})
	})
}
