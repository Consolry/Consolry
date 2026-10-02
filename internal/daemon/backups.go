package daemon

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var validBackup = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}\.zip$`)

type backupInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Created int64  `json:"created"`
	// Skipped lists files a running server had locked, which could not be copied.
	Skipped []string `json:"skipped,omitempty"`
}

func (m *Manager) backupDir(id string) string { return filepath.Join(m.dir, "backups", id) }

// createBackup zips the whole server folder. Backups live outside it, so they survive a restore.
func (m *Manager) createBackup(s *Server) (backupInfo, error) {
	dir := m.backupDir(s.spec.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return backupInfo{}, err
	}
	name := time.Now().UTC().Format("20060102-150405") + ".zip"
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
	return backupInfo{Name: name, Size: info.Size(), Created: info.ModTime().Unix(), Skipped: skipped}, nil
}

// restoreBackup replaces the server folder's contents with the backup's.
func (m *Manager) restoreBackup(s *Server, name string) error {
	if state := s.State(); state == StateRunning || state == StateStopping {
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
		list := []backupInfo{}
		entries, _ := os.ReadDir(m.backupDir(s.spec.ID))
		for _, entry := range entries {
			if info, err := entry.Info(); err == nil && validBackup.MatchString(entry.Name()) {
				list = append(list, backupInfo{Name: entry.Name(), Size: info.Size(), Created: info.ModTime().Unix()})
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Name > list[j].Name })
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /servers/{id}/backups", func(w http.ResponseWriter, r *http.Request) {
		s, ok := server(w, r)
		if !ok {
			return
		}
		info, err := m.createBackup(s)
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

	// java reports the Java runtime on this machine, so the panel can warn before a server fails to start.
	mux.HandleFunc("GET /java", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, detectJava())
	})
}

type javaInfo struct {
	Found   bool   `json:"found"`
	Version string `json:"version"`
	Major   int    `json:"major"`
}

var javaVersion = regexp.MustCompile(`version "([^"]+)"`)

func detectJava() javaInfo {
	out, err := exec.Command("java", "-version").CombinedOutput()
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
