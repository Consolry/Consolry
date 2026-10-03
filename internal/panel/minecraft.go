package panel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DinoNaedYT/Consolry/internal/minecraft"
)

func queryEscape(value string) string            { return url.QueryEscape(value) }
func queryUnescape(value string) (string, error) { return url.QueryUnescape(value) }

// --- version catalogue ---

type versionCache struct {
	mu      sync.Mutex
	fetched map[string]time.Time
	lists   map[string][]string
}

var versions = versionCache{fetched: map[string]time.Time{}, lists: map[string][]string{}}

func (c *versionCache) get(ctx context.Context, software string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if list, ok := c.lists[software]; ok && time.Since(c.fetched[software]) < 15*time.Minute {
		return list, nil
	}
	list, err := minecraft.Versions(ctx, software)
	if err != nil {
		if stale, ok := c.lists[software]; ok {
			return stale, nil
		}
		return nil, err
	}
	c.lists[software], c.fetched[software] = list, time.Now()
	return list, nil
}

func (a *App) handleMinecraftSoftware(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, minecraft.AllSoftware())
}

func (a *App) handleMinecraftVersions(w http.ResponseWriter, r *http.Request) {
	software, ok := minecraft.FindSoftware(r.URL.Query().Get("software"))
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown server software")
		return
	}
	list, err := versions.get(r.Context(), software.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not load the version list: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// --- creating a Minecraft server ---

type minecraftInput struct {
	Software   string `json:"software"`
	Version    string `json:"version"`
	MemoryMB   int    `json:"memoryMb"`
	AcceptEULA bool   `json:"acceptEula"`
}

// FastStartOptions make Java keep an archive of the classes it loaded and reuse it on the next
// start, which measured about 8% faster. The archive lives in the server's cache folder, which
// backups skip. IgnoreUnrecognizedVMOptions keeps older Java versions from refusing to start.
var FastStartOptions = []string{"-XX:+IgnoreUnrecognizedVMOptions", "-XX:+AutoCreateSharedArchive", "-XX:SharedArchiveFile=cache/consolry.jsa"}

// ColourOptions make Paper and its relatives keep colouring their output when it goes to a
// panel instead of a terminal: the first turns colour on, the second lets messages such as the
// answer to /plugins use their full colours.
var ColourOptions = []string{"-Dterminal.ansi=true", "-Dnet.kyori.ansi.colorLevel=truecolor"}

// withColourOption adds any missing ColourOptions to a Java command's arguments, before "-jar".
func withColourOption(args []string) ([]string, bool) {
	jar := -1
	for i, arg := range args {
		if arg == "-jar" {
			jar = i
			break
		}
	}
	if jar < 0 {
		return args, false
	}
	var missing []string
	for _, option := range ColourOptions {
		name := option[:strings.Index(option, "=")]
		found := false
		for _, arg := range args[:jar] {
			if strings.HasPrefix(arg, name) {
				found = true
			}
		}
		if !found {
			missing = append(missing, option)
		}
	}
	if len(missing) == 0 {
		return args, false
	}
	out := append([]string{}, args[:jar]...)
	out = append(out, missing...)
	return append(out, args[jar:]...), true
}

// minecraftSpec works out how to run a Minecraft server and what must be downloaded first.
func minecraftSpec(ctx context.Context, id string, input minecraftInput) (daemonSpec, minecraft.Download, error) {
	if _, ok := minecraft.FindSoftware(input.Software); !ok {
		return daemonSpec{}, minecraft.Download{}, errors.New("choose Paper, Purpur or Fabric")
	}
	if !input.AcceptEULA {
		return daemonSpec{}, minecraft.Download{}, errors.New("you must accept the Minecraft EULA to create a Minecraft server")
	}
	if input.MemoryMB < 512 || input.MemoryMB > 262144 {
		return daemonSpec{}, minecraft.Download{}, errors.New("memory must be between 512 MB and 256 GB")
	}
	download, err := minecraft.Resolve(ctx, input.Software, input.Version)
	if err != nil {
		return daemonSpec{}, minecraft.Download{}, fmt.Errorf("could not find that server version: %w", err)
	}
	memory := strconv.Itoa(input.MemoryMB) + "M"
	args := []string{"-Xms" + memory, "-Xmx" + memory}
	if input.Software != "fabric" {
		args = append(args, FastStartOptions...)
	}
	args = append(args, ColourOptions...)
	spec := daemonSpec{
		ID:          id,
		Command:     "java",
		Args:        append(args, "-jar", "server.jar", "nogui"),
		StopCommand: "stop",
		Java:        download.JavaMin,
	}
	return spec, download, nil
}

// installMinecraft downloads the server jar and records the EULA acceptance on the node.
func installMinecraft(ctx context.Context, node Node, id string, download minecraft.Download) error {
	fetch := map[string]string{"url": download.URL, "path": "server.jar", "sha256": download.SHA256}
	if err := node.slow(ctx, http.MethodPost, "/servers/"+id+"/files/fetch", fetch, nil); err != nil {
		return fmt.Errorf("could not download the server: %w", err)
	}
	eula := "# Accepted through the Consolry panel when this server was created.\n# https://aka.ms/MinecraftEULA\neula=true\n"
	return node.putFile(ctx, id, "eula.txt", []byte(eula))
}

// --- plugins and mods ---

// minecraftServer resolves a request's server and insists it is a Minecraft one.
func (a *App) minecraftServer(w http.ResponseWriter, r *http.Request) (ServerRow, Node, minecraft.Software, bool) {
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return row, node, minecraft.Software{}, false
	}
	software, known := minecraft.FindSoftware(row.Software)
	if row.Kind != "minecraft" || !known {
		writeError(w, http.StatusBadRequest, "this is not a Minecraft server")
		return row, node, minecraft.Software{}, false
	}
	return row, node, software, true
}

type hashedFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	SHA1 string `json:"sha1"`
}

func installedFiles(ctx context.Context, node Node, id, folder string) ([]hashedFile, error) {
	var files []hashedFile
	err := node.slow(ctx, http.MethodGet, "/servers/"+id+"/files/hashes?suffix=.jar&path="+queryEscape(folder), nil, &files)
	return files, err
}

type pluginView struct {
	File      string `json:"file"`
	Size      int64  `json:"size"`
	Verified  bool   `json:"verified"`
	ProjectID string `json:"projectId,omitempty"`
	Title     string `json:"title,omitempty"`
	IconURL   string `json:"iconUrl,omitempty"`
	Version   string `json:"version,omitempty"`
	Update    string `json:"update,omitempty"`
	// Source is where it was installed from, for the ones Consolry recognises.
	Source string `json:"source,omitempty"`
}

func (a *App) handlePlugins(w http.ResponseWriter, r *http.Request) {
	row, node, software, ok := a.minecraftServer(w, r)
	if !ok {
		return
	}
	files, err := installedFiles(r.Context(), node, row.ID, software.Folder)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	sha1s := make([]string, len(files))
	for i, file := range files {
		sha1s[i] = file.SHA1
	}

	// If Modrinth can't be reached the files are still listed, just not identified.
	known, lookupErr := minecraft.Identify(r.Context(), sha1s)
	updates, _ := minecraft.Updates(r.Context(), sha1s, software, row.MCVersion)
	var projectIDs []string
	for _, version := range known {
		projectIDs = append(projectIDs, version.ProjectID)
	}
	titles, _ := minecraft.Titles(r.Context(), projectIDs)

	items := make([]pluginView, len(files))
	for i, file := range files {
		view := pluginView{File: file.Name, Size: file.Size}
		if version, ok := known[file.SHA1]; ok {
			view.Verified, view.ProjectID, view.Version = true, version.ProjectID, version.VersionNumber
			view.Title, view.IconURL, view.Source = titles[version.ProjectID].Title, titles[version.ProjectID].IconURL, minecraft.SourceModrinth
			if newer, ok := updates[file.SHA1]; ok && newer.ID != version.ID {
				view.Update = newer.VersionNumber
			}
		}
		items[i] = view
	}
	a.describeRecorded(r.Context(), row, software, files, items)
	writeJSON(w, http.StatusOK, map[string]any{"folder": software.Folder, "items": items, "lookupFailed": lookupErr != nil})
}

func (a *App) handlePluginSearch(w http.ResponseWriter, r *http.Request) {
	row, _, software, ok := a.minecraftServer(w, r)
	if !ok {
		return
	}
	const pageSize = 16
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 0 || page > 500 {
		page = 0
	}
	query := r.URL.Query().Get("q")
	sources := []string{minecraft.SourceModrinth, minecraft.SourceCurseForge}
	if minecraft.HangarSupports(software) {
		sources = []string{minecraft.SourceModrinth, minecraft.SourceHangar, minecraft.SourceCurseForge}
	}
	reply := map[string]any{"pageSize": pageSize, "sources": sources, "curseforgeReady": a.curseKey() != ""}

	var projects []minecraft.Project
	var total int
	var err error
	switch source := r.URL.Query().Get("source"); source {
	case minecraft.SourceHangar:
		projects, total, err = minecraft.HangarSearch(r.Context(), query, software, row.MCVersion, page*pageSize, pageSize)
	case minecraft.SourceCurseForge:
		if a.curseKey() == "" {
			reply["projects"], reply["total"] = []minecraft.Project{}, 0
			writeJSON(w, http.StatusOK, reply)
			return
		}
		projects, total, err = minecraft.CurseSearch(r.Context(), a.curseKey(), query, software, row.MCVersion, page*pageSize, pageSize)
	default:
		projects, total, err = minecraft.Search(r.Context(), query, software, row.MCVersion, page*pageSize, pageSize)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not search: "+err.Error())
		return
	}
	if projects == nil {
		projects = []minecraft.Project{}
	}
	reply["projects"], reply["total"] = projects, total
	writeJSON(w, http.StatusOK, reply)
}

// installProject installs the newest suitable version of a project, then anything it requires.
func installProject(ctx context.Context, node Node, row ServerRow, software minecraft.Software, projectID string, have map[string]bool, installed *[]string) error {
	if have[projectID] {
		return nil
	}
	have[projectID] = true
	version, found, err := minecraft.Latest(ctx, projectID, software, row.MCVersion)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("there is no version for %s %s", software.Name, row.MCVersion)
	}
	file, ok := version.File()
	if !ok {
		return errors.New("that version has no file to download")
	}
	fetch := map[string]string{"url": file.URL, "path": software.Folder + "/" + file.Filename, "sha512": file.SHA512}
	if err := node.slow(ctx, http.MethodPost, "/servers/"+row.ID+"/files/fetch", fetch, nil); err != nil {
		return err
	}
	*installed = append(*installed, file.Filename)
	for _, dependency := range version.RequiredProjects() {
		if err := installProject(ctx, node, row, software, dependency, have, installed); err != nil {
			return fmt.Errorf("a plugin it depends on could not be installed: %w", err)
		}
	}
	return nil
}

func (a *App) handlePluginInstall(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ProjectID string `json:"projectId"`
		Source    string `json:"source"`
		Title     string `json:"title"`
		IconURL   string `json:"iconUrl"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	row, node, software, ok := a.minecraftServer(w, r)
	if !ok {
		return
	}
	// Anything already installed and recognised is not installed a second time as a dependency.
	have := map[string]bool{}
	if files, err := installedFiles(r.Context(), node, row.ID, software.Folder); err == nil {
		sha1s := make([]string, len(files))
		for i, file := range files {
			sha1s[i] = file.SHA1
		}
		if known, err := minecraft.Identify(r.Context(), sha1s); err == nil {
			for _, version := range known {
				have[version.ProjectID] = true
			}
		}
	}
	delete(have, input.ProjectID)

	if err := backupFirst(r.Context(), node, row.ID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	installed := []string{}
	if input.Source == minecraft.SourceHangar || input.Source == minecraft.SourceCurseForge {
		if err := a.installFromSource(r.Context(), node, row, software, input.Source, input.ProjectID, input.Title, input.IconURL, map[string]bool{}, &installed); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		a.log(r, row.ID, "Installed "+strings.Join(installed, ", ")+" from "+input.Source)
		writeJSON(w, http.StatusOK, map[string]any{"installed": installed})
		return
	}
	if err := installProject(r.Context(), node, row, software, input.ProjectID, have, &installed); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	a.log(r, row.ID, "Installed "+strings.Join(installed, ", "))
	writeJSON(w, http.StatusOK, map[string]any{"installed": installed})
}

func (a *App) handlePluginUpdate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		File string `json:"file"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	row, node, software, ok := a.minecraftServer(w, r)
	if !ok {
		return
	}
	files, err := installedFiles(r.Context(), node, row.ID, software.Folder)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var current hashedFile
	for _, file := range files {
		if file.Name == input.File {
			current = file
		}
	}
	if current.Name == "" {
		writeError(w, http.StatusNotFound, "that file is not installed")
		return
	}
	// Plugins from Hangar or CurseForge are updated from there.
	if _, recorded := a.store.installRecords(row.ID)[strings.ToLower(current.SHA1)]; recorded {
		filename, err := a.updateFromSource(r.Context(), node, row, software, current)
		var refused errConflict
		switch {
		case errors.As(err, &refused):
			writeError(w, http.StatusConflict, err.Error())
		case err != nil:
			writeError(w, http.StatusBadGateway, err.Error())
		default:
			a.log(r, row.ID, "Updated "+current.Name+" to "+filename)
			writeJSON(w, http.StatusOK, map[string]string{"file": filename})
		}
		return
	}
	updates, err := minecraft.Updates(r.Context(), []string{current.SHA1}, software, row.MCVersion)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not check Modrinth: "+err.Error())
		return
	}
	file, ok := updates[current.SHA1].File()
	if !ok || file.SHA1 == current.SHA1 {
		writeError(w, http.StatusConflict, "this is already the newest version for this server")
		return
	}
	if err := backupFirst(r.Context(), node, row.ID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	fetch := map[string]string{"url": file.URL, "path": software.Folder + "/" + file.Filename, "sha512": file.SHA512}
	if err := node.slow(r.Context(), http.MethodPost, "/servers/"+row.ID+"/files/fetch", fetch, nil); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// Remove the old jar only once the new one is safely in place.
	if file.Filename != current.Name {
		_ = node.call(r.Context(), http.MethodDelete, "/servers/"+row.ID+"/files?path="+queryEscape(software.Folder+"/"+current.Name), nil, nil)
	}
	a.log(r, row.ID, "Updated "+current.Name+" to "+file.Filename)
	writeJSON(w, http.StatusOK, map[string]string{"file": file.Filename})
}

// --- crash explainer ---

func (a *App) handleDiagnosis(w http.ResponseWriter, r *http.Request) {
	row, node, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	var lines []string
	if err := node.call(r.Context(), http.MethodGet, "/servers/"+row.ID+"/console/history", nil, &lines); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, minecraft.Explain(lines))
}
