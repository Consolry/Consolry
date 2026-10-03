package daemon

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A backup's name is its UTC time, plus a marker when it was not made by hand.
var validBackup = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}(-auto|-scheduled)?\.zip$`)

// autoReuse is how recent an automatic backup must be to count for the next change too,
// so installing five plugins in a row does not copy the world five times.
const autoReuse = 10 * time.Minute

type backupInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Created int64  `json:"created"`
	// Kind is "manual", "auto" (taken before a change) or "scheduled".
	Kind string `json:"kind"`
	// Skipped lists files a running server had locked, which could not be copied.
	Skipped []string `json:"skipped,omitempty"`
}

// rebuilt names top-level folders the server software downloads or generates again by itself.
// They are left out of backups, which keeps a fresh server's backup small, and a restore leaves them alone.
var rebuilt = map[string]bool{"cache": true, "libraries": true, "versions": true, ".fabric": true}

func (m *Manager) backupDir(id string) string { return filepath.Join(m.dir, "backups", id) }

// createBackup zips the whole server folder. Backups live outside it, so they survive a restore.
func (m *Manager) createBackup(s *Server, kind string) (backupInfo, error) {
	dir := m.backupDir(s.spec.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return backupInfo{}, err
	}
	suffix := ""
	if kind == "auto" || kind == "scheduled" {
		suffix = "-" + kind
	}
	name := time.Now().UTC().Format("20060102-150405") + suffix + ".zip"
	tmp := filepath.Join(dir, name+".part")
	out, err := os.Create(tmp)
	if err != nil {
		return backupInfo{}, err
	}

	root, err := s.root()
	if err != nil {
		out.Close()
		os.Remove(tmp)
		return backupInfo{}, err
	}
	defer root.Close()

	var skipped []string
	archive := zip.NewWriter(out)
	err = fs.WalkDir(root.FS(), ".", func(p string, entry fs.DirEntry, err error) error {
		if err != nil || p == "." {
			return err
		}
		if entry.IsDir() && rebuilt[p] {
			return fs.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return nil // the file vanished while we were walking; skip it
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = p
		if entry.IsDir() {
			header.Name += "/"
			_, err = archive.CreateHeader(header)
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		// Read the start of the file before adding it, so a file the running server has
		// locked (Minecraft does this with session.lock) is left out rather than failing the backup.
		file, err := root.Open(p)
		if err != nil {
			skipped = append(skipped, p)
			return nil
		}
		defer file.Close()
		head := make([]byte, 64*1024)
		n, err := io.ReadFull(file, head)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			skipped = append(skipped, p)
			return nil
		}
		header.Method = zip.Deflate
		w, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := w.Write(head[:n]); err != nil {
			return err
		}
		if _, err := io.Copy(w, file); err != nil {
			skipped = append(skipped, p)
		}
		return nil
	})
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(tmp)
		return backupInfo{}, err
	}
	final := filepath.Join(dir, name)
	if err := os.Rename(tmp, final); err != nil {
		return backupInfo{}, err
	}
	info, err := os.Stat(final)
	if err != nil {
		return backupInfo{}, err
	}
	return backupInfo{Name: name, Size: info.Size(), Created: info.ModTime().Unix(), Kind: backupKind(name), Skipped: skipped}, nil
}

func backupKind(name string) string {
	switch {
	case strings.HasSuffix(name, "-auto.zip"):
		return "auto"
	case strings.HasSuffix(name, "-scheduled.zip"):
		return "scheduled"
	}
	return "manual"
}

// listBackups returns a server's backups, newest first.
func (m *Manager) listBackups(id string) []backupInfo {
	list := []backupInfo{}
	entries, _ := os.ReadDir(m.backupDir(id))
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil && validBackup.MatchString(entry.Name()) {
			list = append(list, backupInfo{Name: entry.Name(), Size: info.Size(), Created: info.ModTime().Unix(), Kind: backupKind(entry.Name())})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name > list[j].Name })
	return list
}

// prune deletes the oldest backups of one kind beyond the newest keep. Manual backups are never pruned.
func (m *Manager) prune(id, kind string, keep int) {
	if kind == "manual" || keep < 1 {
		return
	}
	kept := 0
	for _, backup := range m.listBackups(id) {
		if backup.Kind != kind {
			continue
		}
		if kept++; kept > keep {
			os.Remove(filepath.Join(m.backupDir(id), backup.Name))
		}
	}
}

// restoreBackup replaces the server folder's contents with the backup's.
func (m *Manager) restoreBackup(s *Server, name string) error {
	if state := s.State(); state != StateOffline && state != StateCrashed {
		return errors.New("stop the server before restoring a backup")
	}
	archive, err := zip.OpenReader(filepath.Join(m.backupDir(s.spec.ID), name))
	if err != nil {
		return err
	}
	defer archive.Close()

	root, err := s.root()
	if err != nil {
		return err
	}
	defer root.Close()

	existing, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range existing {
		if entry.IsDir() && rebuilt[entry.Name()] {
			continue
		}
		if err := root.RemoveAll(entry.Name()); err != nil {
			return err
		}
	}

	for _, item := range archive.File {
		rel, err := cleanRel(item.Name)
		if err != nil || rel == "." {
			continue
		}
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
		// root refuses any entry that tries to climb out of the server folder.
		err = writeFile(root, rel, src)
		src.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func registerBackups(mux *http.ServeMux, m *Manager) {
	server := func(w http.ResponseWriter, r *http.Request) (*Server, bool) {
		s, err := m.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, statusFor(err), err.Error())
			return nil, false
		}
		return s, true
	}
	named := func(w http.ResponseWriter, r *http.Request) (*Server, string, bool) {
		s, ok := server(w, r)
		if !ok {
			return nil, "", false
		}
		name := r.PathValue("name")
		if !validBackup.MatchString(name) {
			writeError(w, http.StatusNotFound, "backup not found")
			return nil, "", false
		}
		return s, name, true
	}

	mux.HandleFunc("GET /servers/{id}/backups", func(w http.ResponseWriter, r *http.Request) {
		s, ok := server(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, m.listBackups(s.spec.ID))
	})

	mux.HandleFunc("POST /servers/{id}/backups", func(w http.ResponseWriter, r *http.Request) {
		s, ok := server(w, r)
		if !ok {
			return
		}
		kind := r.URL.Query().Get("kind")
		if kind != "auto" && kind != "scheduled" {
			kind = "manual"
		}
		if kind == "auto" {
			for _, recent := range m.listBackups(s.spec.ID) {
				if recent.Kind == "auto" && time.Since(time.Unix(recent.Created, 0)) < autoReuse {
					writeJSON(w, http.StatusOK, recent)
					return
				}
			}
		}
		info, err := m.createBackup(s, kind)
		if err == nil {
			keep, _ := strconv.Atoi(r.URL.Query().Get("keep"))
			m.prune(s.spec.ID, kind, keep)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, tidyError(err))
			return
		}
		writeJSON(w, http.StatusCreated, info)
	})

	mux.HandleFunc("GET /servers/{id}/backups/{name}", func(w http.ResponseWriter, r *http.Request) {
		s, name, ok := named(w, r)
		if !ok {
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", s.spec.ID+"-"+name))
		http.ServeFile(w, r, filepath.Join(m.backupDir(s.spec.ID), name))
	})

	// Putting a backup back, such as one brought home from off-site storage, so it can be restored.
	mux.HandleFunc("PUT /servers/{id}/backups/{name}", func(w http.ResponseWriter, r *http.Request) {
		s, name, ok := named(w, r)
		if !ok {
			return
		}
		dir := m.backupDir(s.spec.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			writeError(w, http.StatusInternalServerError, tidyError(err))
			return
		}
		tmp := filepath.Join(dir, name+".part")
		out, err := os.Create(tmp)
		if err != nil {
			writeError(w, http.StatusInternalServerError, tidyError(err))
			return
		}
		_, err = io.Copy(out, r.Body)
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err == nil {
			// Only a complete zip is kept.
			var archive *zip.ReadCloser
			if archive, err = zip.OpenReader(tmp); err == nil {
				archive.Close()
			}
		}
		if err != nil {
			_ = os.Remove(tmp)
			writeError(w, http.StatusBadRequest, "that is not a complete backup")
			return
		}
		if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
			_ = os.Remove(tmp)
			writeError(w, http.StatusInternalServerError, tidyError(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("DELETE /servers/{id}/backups/{name}", func(w http.ResponseWriter, r *http.Request) {
		s, name, ok := named(w, r)
		if !ok {
			return
		}
		if err := os.Remove(filepath.Join(m.backupDir(s.spec.ID), name)); err != nil {
			writeError(w, http.StatusNotFound, "backup not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /servers/{id}/backups/{name}/restore", func(w http.ResponseWriter, r *http.Request) {
		s, name, ok := named(w, r)
		if !ok {
			return
		}
		if err := m.restoreBackup(s, name); err != nil {
			status := http.StatusConflict
			if errors.Is(err, fs.ErrNotExist) {
				status = http.StatusNotFound
			}
			writeError(w, status, tidyError(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

}
