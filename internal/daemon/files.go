package daemon

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	maxUpload = 4 << 30 // 4 GB
	userAgent = "Consolry (github.com/DinoNaedYT/Consolry)"
)

var fetchClient = &http.Client{Timeout: 15 * time.Minute}

// root opens the server's folder so that no path, symlink included, can reach outside it.
func (s *Server) root() (*os.Root, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	return os.OpenRoot(s.dir)
}

// cleanRel turns a path from a request into a tidy path relative to the server folder.
func cleanRel(p string) (string, error) {
	p = path.Clean("/" + strings.ReplaceAll(p, "\\", "/"))
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ".", nil
	}
	if strings.ContainsRune(p, 0) {
		return "", errors.New("invalid path")
	}
	return filepath.FromSlash(p), nil
}

type fileEntry struct {
	Name     string `json:"name"`
	Dir      bool   `json:"dir"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
}

// writeFile stores a stream at rel, writing to a temporary name first so a failed
// transfer never leaves a half-written file in place of a good one.
func writeFile(root *os.Root, rel string, src io.Reader, hashes ...hash.Hash) error {
	if dir := filepath.Dir(rel); dir != "." {
		if err := root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := rel + ".consolry-part"
	file, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	writers := []io.Writer{file}
	for _, h := range hashes {
		writers = append(writers, h)
	}
	_, err = io.Copy(io.MultiWriter(writers...), src)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = root.Remove(tmp)
		return err
	}
	return root.Rename(tmp, rel)
}

func registerFiles(mux *http.ServeMux, m *Manager) {
	// open resolves the server and path for a request, answering the error itself if either is bad.
	open := func(w http.ResponseWriter, r *http.Request, rawPath string) (*os.Root, string, bool) {
		s, err := m.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, statusFor(err), err.Error())
			return nil, "", false
		}
		rel, err := cleanRel(rawPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return nil, "", false
		}
		root, err := s.root()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return nil, "", false
		}
		return root, rel, true
	}
	fail := func(w http.ResponseWriter, err error) {
		status := http.StatusBadRequest
		if errors.Is(err, fs.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, tidyError(err))
	}

	mux.HandleFunc("GET /servers/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		root, rel, ok := open(w, r, r.URL.Query().Get("path"))
		if !ok {
			return
		}
		defer root.Close()
		dir, err := root.Open(rel)
		if err != nil {
			fail(w, err)
			return
		}
		defer dir.Close()
		items, err := dir.ReadDir(-1)
		if err != nil {
			fail(w, err)
			return
		}
		entries := make([]fileEntry, 0, len(items))
		for _, item := range items {
			info, err := item.Info()
			if err != nil || strings.HasSuffix(item.Name(), ".consolry-part") {
				continue
			}
			entries = append(entries, fileEntry{Name: item.Name(), Dir: item.IsDir(), Size: info.Size(), Modified: info.ModTime().Unix()})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Dir != entries[j].Dir {
				return entries[i].Dir
			}
			return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		})
		writeJSON(w, http.StatusOK, entries)
	})

	mux.HandleFunc("GET /servers/{id}/files/content", func(w http.ResponseWriter, r *http.Request) {
		root, rel, ok := open(w, r, r.URL.Query().Get("path"))
		if !ok {
			return
		}
		defer root.Close()
		file, err := root.Open(rel)
		if err != nil {
			fail(w, err)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || info.IsDir() {
			writeError(w, http.StatusBadRequest, "that is a folder, not a file")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
		http.ServeContent(w, r, "", info.ModTime(), file)
	})

	mux.HandleFunc("PUT /servers/{id}/files/content", func(w http.ResponseWriter, r *http.Request) {
		root, rel, ok := open(w, r, r.URL.Query().Get("path"))
		if !ok {
			return
		}
		defer root.Close()
		if rel == "." {
			writeError(w, http.StatusBadRequest, "a file name is required")
			return
		}
		if err := writeFile(root, rel, http.MaxBytesReader(w, r.Body, maxUpload)); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /servers/{id}/files/mkdir", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Path string `json:"path"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&input) != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		root, rel, ok := open(w, r, input.Path)
		if !ok {
			return
		}
		defer root.Close()
		if err := root.MkdirAll(rel, 0o755); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /servers/{id}/files/rename", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&input) != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		root, from, ok := open(w, r, input.From)
		if !ok {
			return
		}
		defer root.Close()
		to, err := cleanRel(input.To)
		if err != nil || from == "." || to == "." {
			writeError(w, http.StatusBadRequest, "both the old and the new name are required")
			return
		}
		if _, err := root.Lstat(to); err == nil {
			writeError(w, http.StatusConflict, "something with that name already exists")
			return
		}
		if err := root.Rename(from, to); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("DELETE /servers/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		root, rel, ok := open(w, r, r.URL.Query().Get("path"))
		if !ok {
			return
		}
		defer root.Close()
		if rel == "." {
			writeError(w, http.StatusBadRequest, "choose a file or folder to delete")
			return
		}
		if err := root.RemoveAll(rel); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// fetch downloads a file from the internet straight onto the node, checking its fingerprint.
	mux.HandleFunc("POST /servers/{id}/files/fetch", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			URL    string `json:"url"`
			Path   string `json:"path"`
			SHA1   string `json:"sha1"`
			SHA256 string `json:"sha256"`
			SHA512 string `json:"sha512"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&input) != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if !strings.HasPrefix(input.URL, "https://") {
			writeError(w, http.StatusBadRequest, "downloads must use https")
			return
		}
		root, rel, ok := open(w, r, input.Path)
		if !ok {
			return
		}
		defer root.Close()
		if rel == "." {
			writeError(w, http.StatusBadRequest, "a file name is required")
			return
		}

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, input.URL, nil)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid download address")
			return
		}
		req.Header.Set("User-Agent", userAgent)
		res, err := fetchClient.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, "download failed: "+err.Error())
			return
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			writeError(w, http.StatusBadGateway, "download failed: the server answered "+res.Status)
			return
		}

		h1, h256, h512 := sha1.New(), sha256.New(), sha512.New()
		if err := writeFile(root, rel, io.LimitReader(res.Body, maxUpload), h1, h256, h512); err != nil {
			fail(w, err)
			return
		}
		for want, got := range map[string]hash.Hash{input.SHA1: h1, input.SHA256: h256, input.SHA512: h512} {
			if want != "" && !strings.EqualFold(want, hex.EncodeToString(got.Sum(nil))) {
				_ = root.Remove(rel)
				writeError(w, http.StatusBadGateway, "the downloaded file did not match its published fingerprint, so it was discarded")
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// hashes fingerprints the files in one folder so the panel can recognise known plugins.
	mux.HandleFunc("GET /servers/{id}/files/hashes", func(w http.ResponseWriter, r *http.Request) {
		root, rel, ok := open(w, r, r.URL.Query().Get("path"))
		if !ok {
			return
		}
		defer root.Close()
		suffix := r.URL.Query().Get("suffix")
		type hashed struct {
			Name   string `json:"name"`
			Size   int64  `json:"size"`
			SHA1   string `json:"sha1"`
			SHA512 string `json:"sha512"`
		}
		result := []hashed{}
		dir, err := root.Open(rel)
		if errors.Is(err, fs.ErrNotExist) {
			writeJSON(w, http.StatusOK, result)
			return
		}
		if err != nil {
			fail(w, err)
			return
		}
		items, _ := dir.ReadDir(-1)
		dir.Close()
		for _, item := range items {
			if item.IsDir() || !strings.HasSuffix(strings.ToLower(item.Name()), suffix) {
				continue
			}
			file, err := root.Open(filepath.Join(rel, item.Name()))
			if err != nil {
				continue
			}
			h1, h512 := sha1.New(), sha512.New()
			size, err := io.Copy(io.MultiWriter(h1, h512), file)
			file.Close()
			if err != nil {
				continue
			}
			result = append(result, hashed{Name: item.Name(), Size: size, SHA1: hex.EncodeToString(h1.Sum(nil)), SHA512: hex.EncodeToString(h512.Sum(nil))})
		}
		sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /servers/{id}/console/history", func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, statusFor(err), err.Error())
			return
		}
		history, _, cancel := s.Subscribe()
		cancel()
		// The panel reads this to match patterns, so the colour codes are taken out.
		for i, line := range history {
			history[i] = Plain(line)
		}
		writeJSON(w, http.StatusOK, history)
	})
}

// tidyError drops the on-disk location from a file error so node paths don't reach the browser.
func tidyError(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Op + ": " + pathErr.Err.Error()
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.Op + ": " + linkErr.Err.Error()
	}
	return err.Error()
}
