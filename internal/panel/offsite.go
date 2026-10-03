package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	s3creds "github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage is an S3-compatible bucket that backups are copied to, so a dead disk or a
// stolen computer does not take every copy of a world with it. Backblaze B2, Cloudflare R2,
// Wasabi, Amazon S3 and MinIO all speak this.
type Storage struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"accessKey"`
	Secret    string `json:"secret,omitempty"`
	// Folder is put in front of every copy's name, so one bucket can serve several panels.
	Folder string `json:"folder"`
	// Keep is how many copies of each server are kept; older ones are deleted.
	Keep int `json:"keep"`
}

func (s *Store) storage() Storage {
	var storage Storage
	_ = json.Unmarshal([]byte(s.Setting("storage", "{}")), &storage)
	if storage.Keep == 0 {
		storage.Keep = 7
	}
	return storage
}

func (st Storage) ready() bool {
	return st.Endpoint != "" && st.Bucket != "" && st.AccessKey != "" && st.Secret != ""
}

func (st Storage) client() (*minio.Client, error) {
	endpoint := strings.TrimSpace(st.Endpoint)
	secure := true
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
		endpoint, secure = parsed.Host, parsed.Scheme != "http"
	}
	return minio.New(endpoint, &minio.Options{
		Creds:  s3creds.NewStaticV4(st.AccessKey, st.Secret, ""),
		Secure: secure,
		Region: st.Region,
	})
}

func (st Storage) prefix(serverID string) string {
	folder := strings.Trim(st.Folder, "/")
	if folder == "" {
		folder = "consolry"
	}
	return folder + "/" + serverID + "/"
}

// offsiteOn reports whether a server's backups are copied off-site.
func (a *App) offsiteOn(serverID string) bool {
	return a.store.Setting("offsite:"+serverID, "") == "on" && a.store.storage().ready()
}

// copyOffsite streams one of a server's backups from its node to the bucket, then
// deletes copies beyond the number to keep.
func (a *App) copyOffsite(ctx context.Context, node Node, row ServerRow, name string) error {
	storage := a.store.storage()
	if !storage.ready() {
		return errors.New("off-site storage is not set up")
	}
	client, err := storage.client()
	if err != nil {
		return err
	}
	res, err := node.raw(ctx, http.MethodGet, "/servers/"+row.ID+"/backups/"+name, "", nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err := daemonError(res); err != nil {
		return err
	}
	if _, err := client.PutObject(ctx, storage.Bucket, storage.prefix(row.ID)+name, res.Body, res.ContentLength,
		minio.PutObjectOptions{ContentType: "application/zip"}); err != nil {
		return fmt.Errorf("the storage refused the copy: %w", err)
	}
	copies, err := a.offsiteCopies(ctx, row.ID)
	if err != nil {
		return nil
	}
	for i := storage.Keep; i < len(copies); i++ {
		_ = client.RemoveObject(ctx, storage.Bucket, storage.prefix(row.ID)+copies[i].Name, minio.RemoveObjectOptions{})
	}
	return nil
}

// copyOffsiteLater copies a fresh backup in the background, if the server asks for that,
// and says how it went in the activity log.
func (a *App) copyOffsiteLater(node Node, row ServerRow, name string) {
	if !a.offsiteOn(row.ID) || name == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		defer cancel()
		if err := a.copyOffsite(ctx, node, row, name); err != nil {
			a.store.Log(row.ID, "consolry", "Could not copy backup "+name+" off-site: "+err.Error())
			a.notify(row.ID, "task", "A backup of "+row.Name+" was not copied off-site", err.Error())
			return
		}
		a.store.Log(row.ID, "consolry", "Copied backup "+name+" off-site")
	}()
}

type offsiteCopy struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Created int64  `json:"created"`
}

var backupName = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}(-auto|-scheduled)?\.zip$`)

// offsiteCopies lists a server's copies in the bucket, newest first.
func (a *App) offsiteCopies(ctx context.Context, serverID string) ([]offsiteCopy, error) {
	storage := a.store.storage()
	client, err := storage.client()
	if err != nil {
		return nil, err
	}
	copies := []offsiteCopy{}
	for object := range client.ListObjects(ctx, storage.Bucket, minio.ListObjectsOptions{Prefix: storage.prefix(serverID)}) {
		if object.Err != nil {
			return nil, object.Err
		}
		name := strings.TrimPrefix(object.Key, storage.prefix(serverID))
		if backupName.MatchString(name) {
			copies = append(copies, offsiteCopy{Name: name, Size: object.Size, Created: object.LastModified.Unix()})
		}
	}
	// Names start with the time they were taken, so they sort by age.
	sort.Slice(copies, func(i, j int) bool { return copies[i].Name > copies[j].Name })
	return copies, nil
}

// --- the Backups tab ---

func (a *App) handleOffsite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	storage := a.store.storage()
	reply := map[string]any{"available": storage.ready(), "enabled": a.offsiteOn(id), "keep": storage.Keep, "copies": []offsiteCopy{}}
	if storage.ready() {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		copies, err := a.offsiteCopies(ctx, id)
		if err != nil {
			reply["problem"] = "Could not list the off-site copies: " + err.Error()
		} else {
			reply["copies"] = copies
		}
	}
	writeJSON(w, http.StatusOK, reply)
}

func (a *App) handleUpdateOffsite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled *bool  `json:"enabled"`
		CopyNow string `json:"copyNow"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	if !a.store.storage().ready() {
		writeError(w, http.StatusConflict, "off-site storage is not set up yet. The panel's admin sets it up on the Storage page")
		return
	}
	if input.Enabled != nil {
		value := ""
		if *input.Enabled {
			value = "on"
		}
		if err := a.store.SetSetting("offsite:"+row.ID, value); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if input.CopyNow != "" {
		if !backupName.MatchString(input.CopyNow) {
			writeError(w, http.StatusBadRequest, "backup not found")
			return
		}
		if err := a.copyOffsite(r.Context(), node, row, input.CopyNow); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		a.log(r, row.ID, "Copied backup "+input.CopyNow+" off-site")
	}
	a.handleOffsite(w, r)
}

func (a *App) offsiteName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("name")
	if !backupName.MatchString(name) || !a.store.storage().ready() {
		writeError(w, http.StatusNotFound, "copy not found")
		return "", false
	}
	return name, true
}

// handleOffsiteDownload sends the browser straight to the bucket with a link that works for a few minutes.
func (a *App) handleOffsiteDownload(w http.ResponseWriter, r *http.Request) {
	name, ok := a.offsiteName(w, r)
	if !ok {
		return
	}
	storage := a.store.storage()
	client, err := storage.client()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	params := url.Values{"response-content-disposition": {fmt.Sprintf("attachment; filename=%q", r.PathValue("id")+"-"+name)}}
	link, err := client.PresignedGetObject(r.Context(), storage.Bucket, storage.prefix(r.PathValue("id"))+name, 10*time.Minute, params)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	http.Redirect(w, r, link.String(), http.StatusFound)
}

// handleOffsiteRestore brings a copy back to the server's machine and restores it there.
func (a *App) handleOffsiteRestore(w http.ResponseWriter, r *http.Request) {
	name, ok := a.offsiteName(w, r)
	if !ok {
		return
	}
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	storage := a.store.storage()
	client, err := storage.client()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	object, err := client.GetObject(r.Context(), storage.Bucket, storage.prefix(row.ID)+name, minio.GetObjectOptions{})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer object.Close()
	res, err := node.raw(r.Context(), http.MethodPut, "/servers/"+row.ID+"/backups/"+name, "application/zip", object)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach the node")
		return
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if err := daemonError(res); err != nil {
		writeError(w, http.StatusBadGateway, "could not bring the copy back: "+err.Error())
		return
	}
	if err := node.slow(r.Context(), http.MethodPost, "/servers/"+row.ID+"/backups/"+name+"/restore", nil, nil); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	a.log(r, row.ID, "Restored backup "+name+" from off-site")
	w.WriteHeader(http.StatusNoContent)
}

// --- the admin's Storage page ---

func (a *App) handleStorage(w http.ResponseWriter, r *http.Request) {
	storage := a.store.storage()
	hasSecret := storage.Secret != ""
	storage.Secret = ""
	writeJSON(w, http.StatusOK, map[string]any{"storage": storage, "secretSet": hasSecret})
}

func (a *App) handleUpdateStorage(w http.ResponseWriter, r *http.Request) {
	var input Storage
	if !readJSON(w, r, &input) {
		return
	}
	input.Endpoint, input.Bucket, input.AccessKey = strings.TrimSpace(input.Endpoint), strings.TrimSpace(input.Bucket), strings.TrimSpace(input.AccessKey)
	input.Region, input.Folder = strings.TrimSpace(input.Region), strings.Trim(strings.TrimSpace(input.Folder), "/")
	if input.Keep < 1 || input.Keep > 365 {
		writeError(w, http.StatusBadRequest, "keep between 1 and 365 copies of each server")
		return
	}
	// An empty secret keeps the one already saved.
	if input.Secret == "" {
		input.Secret = a.store.storage().Secret
	}
	if input.Endpoint != "" {
		if !input.ready() {
			writeError(w, http.StatusBadRequest, "fill in the address, bucket, key ID and secret key")
			return
		}
		// Prove the details work by writing and removing a small file.
		client, err := input.client()
		if err != nil {
			writeError(w, http.StatusBadRequest, "that address is not right: "+err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		probe := input.prefix("") + "consolry-check-" + strconv.FormatInt(time.Now().Unix(), 10)
		if _, err := client.PutObject(ctx, input.Bucket, probe, strings.NewReader("ok"), 2, minio.PutObjectOptions{}); err != nil {
			writeError(w, http.StatusBadRequest, "the storage did not accept a test file: "+err.Error())
			return
		}
		_ = client.RemoveObject(ctx, input.Bucket, probe, minio.RemoveObjectOptions{})
	}
	data, _ := json.Marshal(input)
	if err := a.store.SetSetting("storage", string(data)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.handleStorage(w, r)
}
