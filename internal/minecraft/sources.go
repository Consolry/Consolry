package minecraft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Where plugins and mods can be installed from. Modrinth is in modrinth.go.
const (
	SourceModrinth   = "modrinth"
	SourceHangar     = "hangar"
	SourceCurseForge = "curseforge"
)

// Release is one downloadable file from Hangar or CurseForge, with what is needed to check it.
type Release struct {
	Version  string
	URL      string
	Filename string
	SHA1     string
	SHA256   string
	// Requires lists the projects this one needs to run, in the same source.
	Requires []string
}

// ErrNotDownloadable is returned when a project's author does not allow downloads through other programs.
var ErrNotDownloadable = errors.New("its author only allows downloading it from their own site, so it cannot be installed or checked here")

func getWithHeaders(ctx context.Context, address string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%s refused the request (%d). Check the API key", req.URL.Host, res.StatusCode)
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("%s answered %s", req.URL.Host, res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// --- Hangar, PaperMC's plugin site. Paper plugins only. ---

const hangar = "https://hangar.papermc.io/api/v1"

// HangarSupports reports whether Hangar has plugins for this server software.
func HangarSupports(software Software) bool { return software.ModrinthType == "plugin" }

func HangarSearch(ctx context.Context, query string, software Software, gameVersion string, offset, limit int) ([]Project, int, error) {
	if !HangarSupports(software) {
		return nil, 0, errors.New("Hangar only has Paper plugins")
	}
	params := url.Values{
		"q": {query}, "limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)},
		"platform": {"PAPER"}, "version": {gameVersion}, "sort": {"-downloads"},
	}
	var reply struct {
		Pagination struct {
			Count int `json:"count"`
		} `json:"pagination"`
		Result []struct {
			Name      string `json:"name"`
			Namespace struct {
				Owner string `json:"owner"`
				Slug  string `json:"slug"`
			} `json:"namespace"`
			Description string `json:"description"`
			AvatarURL   string `json:"avatarUrl"`
			Stats       struct {
				Downloads int `json:"downloads"`
			} `json:"stats"`
		} `json:"result"`
	}
	if err := getWithHeaders(ctx, hangar+"/projects?"+params.Encode(), nil, &reply); err != nil {
		return nil, 0, err
	}
	projects := make([]Project, 0, len(reply.Result))
	for _, item := range reply.Result {
		projects = append(projects, Project{
			ID: item.Namespace.Slug, Title: item.Name, Description: item.Description,
			Author: item.Namespace.Owner, Downloads: item.Stats.Downloads, IconURL: item.AvatarURL, Source: SourceHangar,
		})
	}
	return projects, reply.Pagination.Count, nil
}

// HangarLatest finds the newest release of a project for this Minecraft version, skipping test builds.
func HangarLatest(ctx context.Context, project string, gameVersion string) (Release, bool, error) {
	params := url.Values{"limit": {"1"}, "platform": {"PAPER"}, "platformVersion": {gameVersion}, "channel": {"Release"}}
	var reply struct {
		Result []struct {
			Name      string `json:"name"`
			Downloads map[string]struct {
				FileInfo *struct {
					Name   string `json:"name"`
					SHA256 string `json:"sha256Hash"`
				} `json:"fileInfo"`
				ExternalURL *string `json:"externalUrl"`
				DownloadURL *string `json:"downloadUrl"`
			} `json:"downloads"`
			Dependencies map[string][]struct {
				ProjectID *int `json:"projectId"`
				Required  bool `json:"required"`
			} `json:"pluginDependencies"`
		} `json:"result"`
	}
	if err := getWithHeaders(ctx, hangar+"/projects/"+url.PathEscape(project)+"/versions?"+params.Encode(), nil, &reply); err != nil {
		return Release{}, false, err
	}
	if len(reply.Result) == 0 {
		return Release{}, false, nil
	}
	version := reply.Result[0]
	download, ok := version.Downloads["PAPER"]
	if !ok || download.DownloadURL == nil || download.FileInfo == nil || download.FileInfo.SHA256 == "" {
		// Hosted elsewhere, so there is no fingerprint to check the file against.
		return Release{}, false, ErrNotDownloadable
	}
	release := Release{Version: version.Name, URL: *download.DownloadURL, Filename: download.FileInfo.Name, SHA256: download.FileInfo.SHA256}
	for _, dependency := range version.Dependencies["PAPER"] {
		if dependency.Required && dependency.ProjectID != nil {
			release.Requires = append(release.Requires, strconv.Itoa(*dependency.ProjectID))
		}
	}
	return release, true, nil
}

// --- CurseForge. Needs an API key, which each panel's admin gets for free from CurseForge. ---

const curseforge = "https://api.curseforge.com/v1"

// CurseForge's numbers for Minecraft, its "Bukkit plugins" and "mods" sections, and the Fabric loader.
const (
	curseMinecraft = 432
	cursePlugins   = 5
	curseMods      = 6
	curseFabric    = 4
)

func curseFilters(software Software, gameVersion string) url.Values {
	params := url.Values{"gameId": {strconv.Itoa(curseMinecraft)}, "gameVersion": {gameVersion}}
	if software.ModrinthType == "mod" {
		params.Set("classId", strconv.Itoa(curseMods))
		params.Set("modLoaderType", strconv.Itoa(curseFabric))
	} else {
		params.Set("classId", strconv.Itoa(cursePlugins))
	}
	return params
}

func CurseSearch(ctx context.Context, key, query string, software Software, gameVersion string, offset, limit int) ([]Project, int, error) {
	if key == "" {
		return nil, 0, errors.New("CurseForge needs an API key")
	}
	params := curseFilters(software, gameVersion)
	params.Set("searchFilter", query)
	params.Set("index", strconv.Itoa(offset))
	params.Set("pageSize", strconv.Itoa(limit))
	params.Set("sortField", "2") // popularity
	params.Set("sortOrder", "desc")
	var reply struct {
		Data []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Summary   string `json:"summary"`
			Downloads int    `json:"downloadCount"`
			Logo      *struct {
				Thumbnail string `json:"thumbnailUrl"`
			} `json:"logo"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
		} `json:"data"`
		Pagination struct {
			Total int `json:"totalCount"`
		} `json:"pagination"`
	}
	if err := getWithHeaders(ctx, curseforge+"/mods/search?"+params.Encode(), map[string]string{"x-api-key": key}, &reply); err != nil {
		return nil, 0, err
	}
	// CurseForge refuses to page past its first 10,000 results.
	total := min(reply.Pagination.Total, 10000)
	projects := make([]Project, 0, len(reply.Data))
	for _, item := range reply.Data {
		project := Project{ID: strconv.Itoa(item.ID), Title: item.Name, Description: item.Summary, Downloads: item.Downloads, Source: SourceCurseForge}
		if item.Logo != nil {
			project.IconURL = item.Logo.Thumbnail
		}
		if len(item.Authors) > 0 {
			project.Author = item.Authors[0].Name
		}
		projects = append(projects, project)
	}
	return projects, total, nil
}

// CurseLatest finds the newest file of a project for this Minecraft version.
func CurseLatest(ctx context.Context, key, project string, software Software, gameVersion string) (Release, bool, error) {
	if key == "" {
		return Release{}, false, errors.New("CurseForge needs an API key")
	}
	params := url.Values{"gameVersion": {gameVersion}, "pageSize": {"1"}}
	if software.ModrinthType == "mod" {
		params.Set("modLoaderType", strconv.Itoa(curseFabric))
	}
	var reply struct {
		Data []struct {
			DisplayName string  `json:"displayName"`
			FileName    string  `json:"fileName"`
			DownloadURL *string `json:"downloadUrl"`
			Hashes      []struct {
				Value string `json:"value"`
				Algo  int    `json:"algo"` // 1 is SHA-1
			} `json:"hashes"`
			Dependencies []struct {
				ModID    int `json:"modId"`
				Relation int `json:"relationType"` // 3 is "required"
			} `json:"dependencies"`
		} `json:"data"`
	}
	if err := getWithHeaders(ctx, curseforge+"/mods/"+url.PathEscape(project)+"/files?"+params.Encode(), map[string]string{"x-api-key": key}, &reply); err != nil {
		return Release{}, false, err
	}
	if len(reply.Data) == 0 {
		return Release{}, false, nil
	}
	file := reply.Data[0]
	release := Release{Version: strings.TrimSuffix(file.DisplayName, ".jar"), Filename: file.FileName}
	for _, hash := range file.Hashes {
		if hash.Algo == 1 {
			release.SHA1 = hash.Value
		}
	}
	if file.DownloadURL == nil || *file.DownloadURL == "" || release.SHA1 == "" {
		return Release{}, false, ErrNotDownloadable
	}
	release.URL = *file.DownloadURL
	for _, dependency := range file.Dependencies {
		if dependency.Relation == 3 {
			release.Requires = append(release.Requires, strconv.Itoa(dependency.ModID))
		}
	}
	return release, true, nil
}
