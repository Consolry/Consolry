package daemon

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

// importClient allows an hour for a big server's files to arrive.
var importClient = &http.Client{Timeout: time.Hour}

// registerImport lets the panel bring a whole server's files in from another panel,
// such as Pterodactyl, as one .tar.gz archive.
func registerImport(mux *http.ServeMux, m *Manager) {
	mux.HandleFunc("POST /servers/{id}/files/import", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			URL string `json:"url"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&input) != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		// Another panel's machines often have no certificate, so plain http is accepted here.
		// Only the panel can call this, and nothing downloaded is run until someone starts the server.
		if !strings.HasPrefix(input.URL, "https://") && !strings.HasPrefix(input.URL, "http://") {
			writeError(w, http.StatusBadRequest, "invalid download address")
			return
		}
		s, err := m.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, statusFor(err), err.Error())
			return
		}
		root, err := s.root()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer root.Close()

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, input.URL, nil)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid download address")
			return
		}
		req.Header.Set("User-Agent", userAgent)
		res, err := importClient.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, "download failed: "+err.Error())
			return
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			writeError(w, http.StatusBadGateway, "download failed: the server answered "+res.Status)
			return
		}

		count, err := extractTarGz(res.Body, func(name string, body io.Reader, dir bool) error {
			if dir {
				return root.MkdirAll(name, 0o755)
			}
			return writeFile(root, name, io.LimitReader(body, maxUpload))
		})
		if err != nil {
			writeError(w, http.StatusBadGateway, "could not unpack the files: "+tidyError(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"files": count})
	})
}

// extractTarGz unpacks an archive, handing each folder and plain file to save.
// Links and anything that would land outside the server's folder are skipped.
func extractTarGz(src io.Reader, save func(name string, body io.Reader, dir bool) error) (int, error) {
	unzipped, err := gzip.NewReader(src)
	if err != nil {
		return 0, errors.New("the download is not a .tar.gz archive")
	}
	defer unzipped.Close()
	archive := tar.NewReader(unzipped)
	count := 0
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return count, nil
		}
		if err != nil {
			return count, err
		}
		name := path.Clean(strings.TrimPrefix(strings.ReplaceAll(header.Name, "\\", "/"), "/"))
		if name == "." || name == ".." || strings.HasPrefix(name, "../") {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			err = save(name, nil, true)
		case tar.TypeReg:
			err = save(name, archive, false)
			count++
		default:
			continue
		}
		if err != nil {
			return count, fmt.Errorf("%s: %w", name, err)
		}
	}
}
