package panel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/DinoNaedYT/Consolry/internal/minecraft"
)

// Plugins installed from Hangar or CurseForge are remembered by their fingerprint, because
// unlike Modrinth those sites cannot be asked "which plugin is this file?".
type installRecord struct {
	Source  string
	Project string
	Version string
	Title   string
	IconURL string
}

func (s *Store) recordInstall(serverID, sha1 string, record installRecord) error {
	_, err := s.db.Exec(`
		INSERT INTO plugin_installs (server_id, sha1, source, project, version, title, icon) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (server_id, sha1) DO UPDATE SET source = excluded.source, project = excluded.project,
			version = excluded.version, title = excluded.title, icon = excluded.icon`,
		serverID, strings.ToLower(sha1), record.Source, record.Project, record.Version, record.Title, record.IconURL)
	return err
}

func (s *Store) installRecords(serverID string) map[string]installRecord {
	records := map[string]installRecord{}
	rows, err := s.db.Query(`SELECT sha1, source, project, version, title, icon FROM plugin_installs WHERE server_id = ?`, serverID)
	if err != nil {
		return records
	}
	defer rows.Close()
	for rows.Next() {
		var sha1 string
		var record installRecord
		if rows.Scan(&sha1, &record.Source, &record.Project, &record.Version, &record.Title, &record.IconURL) == nil {
			records[sha1] = record
		}
	}
	return records
}

// curseKey is the CurseForge API key: the admin's, or one given to the program when it starts.
func (a *App) curseKey() string {
	if key := a.store.Setting("curseforge_key", ""); key != "" {
		return key
	}
	return os.Getenv("CONSOLRY_CURSEFORGE_KEY")
}

// latestRelease asks Hangar or CurseForge for a project's newest file for this server.
func (a *App) latestRelease(ctx context.Context, source, project string, software minecraft.Software, gameVersion string) (minecraft.Release, bool, error) {
	switch source {
	case minecraft.SourceHangar:
		return minecraft.HangarLatest(ctx, project, gameVersion)
	case minecraft.SourceCurseForge:
		return minecraft.CurseLatest(ctx, a.curseKey(), project, software, gameVersion)
	}
	return minecraft.Release{}, false, fmt.Errorf("unknown source %q", source)
}

// fetchRelease downloads a release onto the node, checked against its fingerprint, and remembers it.
func (a *App) fetchRelease(ctx context.Context, node Node, row ServerRow, software minecraft.Software, source, project string, release minecraft.Release, title, icon string) error {
	fetch := map[string]string{"url": release.URL, "path": software.Folder + "/" + release.Filename, "sha1": release.SHA1, "sha256": release.SHA256}
	if err := node.slow(ctx, http.MethodPost, "/servers/"+row.ID+"/files/fetch", fetch, nil); err != nil {
		return err
	}
	files, err := installedFiles(ctx, node, row.ID, software.Folder)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.Name == release.Filename {
			return a.store.recordInstall(row.ID, file.SHA1, installRecord{Source: source, Project: project, Version: release.Version, Title: title, IconURL: icon})
		}
	}
	return nil
}

// installFromSource installs a Hangar or CurseForge project, then anything it requires.
func (a *App) installFromSource(ctx context.Context, node Node, row ServerRow, software minecraft.Software, source, project, title, icon string, have map[string]bool, installed *[]string) error {
	if have[source+"/"+project] {
		return nil
	}
	have[source+"/"+project] = true
	release, found, err := a.latestRelease(ctx, source, project, software, row.MCVersion)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("there is no version for %s %s", software.Name, row.MCVersion)
	}
	if err := a.fetchRelease(ctx, node, row, software, source, project, release, title, icon); err != nil {
		return err
	}
	*installed = append(*installed, release.Filename)
	for _, dependency := range release.Requires {
		if err := a.installFromSource(ctx, node, row, software, source, dependency, "", "", have, installed); err != nil {
			return fmt.Errorf("a plugin it depends on could not be installed: %w", err)
		}
	}
	return nil
}

// describeRecorded fills in plugins that were installed from Hangar or CurseForge, and
// checks those sites for newer versions, a few at a time.
func (a *App) describeRecorded(ctx context.Context, row ServerRow, software minecraft.Software, files []hashedFile, items []pluginView) {
	records := a.store.installRecords(row.ID)
	var wait sync.WaitGroup
	limit := make(chan struct{}, 6)
	for i, file := range files {
		record, ok := records[strings.ToLower(file.SHA1)]
		if !ok || items[i].Verified {
			continue
		}
		items[i].Verified, items[i].ProjectID, items[i].Version = true, record.Project, record.Version
		items[i].Title, items[i].IconURL, items[i].Source = record.Title, record.IconURL, record.Source
		wait.Add(1)
		go func(i int, record installRecord) {
			defer wait.Done()
			limit <- struct{}{}
			defer func() { <-limit }()
			if release, found, err := a.latestRelease(ctx, record.Source, record.Project, software, row.MCVersion); err == nil && found && release.Version != record.Version {
				items[i].Update = release.Version
			}
		}(i, record)
	}
	wait.Wait()
}

// updateFromSource replaces a Hangar or CurseForge plugin with its newest version.
func (a *App) updateFromSource(ctx context.Context, node Node, row ServerRow, software minecraft.Software, current hashedFile) (string, error) {
	record, ok := a.store.installRecords(row.ID)[strings.ToLower(current.SHA1)]
	if !ok {
		return "", errors.New("Consolry does not know where this plugin came from, so it cannot update it")
	}
	release, found, err := a.latestRelease(ctx, record.Source, record.Project, software, row.MCVersion)
	if err != nil {
		return "", err
	}
	if !found || release.Version == record.Version {
		return "", errConflict("this is already the newest version for this server")
	}
	if err := backupFirst(ctx, node, row.ID); err != nil {
		return "", err
	}
	if err := a.fetchRelease(ctx, node, row, software, record.Source, record.Project, release, record.Title, record.IconURL); err != nil {
		return "", err
	}
	if release.Filename != current.Name {
		_ = node.call(ctx, http.MethodDelete, "/servers/"+row.ID+"/files?path="+queryEscape(software.Folder+"/"+current.Name), nil, nil)
	}
	return release.Filename, nil
}

// errConflict marks a refusal that is about the request, not a failure to reach anything.
type errConflict string

func (e errConflict) Error() string { return string(e) }

func (a *App) handlePluginSources(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurseForgeKey string `json:"curseforgeKey"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	key := strings.TrimSpace(input.CurseForgeKey)
	if key != "" {
		// Check the key works before keeping it.
		paper, _ := minecraft.FindSoftware("paper")
		if _, _, err := minecraft.CurseSearch(r.Context(), key, "", paper, "", 0, 1); err != nil {
			writeError(w, http.StatusBadRequest, "CurseForge did not accept that key: "+err.Error())
			return
		}
	}
	if err := a.store.SetSetting("curseforge_key", key); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
