package minecraft

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

const modrinth = "https://api.modrinth.com/v2"

// Project is a plugin or mod as shown in search results.
type Project struct {
	ID          string `json:"projectId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Downloads   int    `json:"downloads"`
	IconURL     string `json:"iconUrl"`
	// Source is the site it comes from: modrinth, hangar or curseforge.
	Source string `json:"source"`
}

// Version is one downloadable release of a project.
type Version struct {
	ID            string `json:"id"`
	ProjectID     string `json:"project_id"`
	VersionNumber string `json:"version_number"`
	// VersionType is "release", "beta" or "alpha". Only releases are offered unless there is nothing else.
	VersionType string `json:"version_type"`
	Files       []struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Primary  bool   `json:"primary"`
		Hashes   struct {
			SHA1   string `json:"sha1"`
			SHA512 string `json:"sha512"`
		} `json:"hashes"`
	} `json:"files"`
	Dependencies []struct {
		ProjectID string `json:"project_id"`
		Type      string `json:"dependency_type"`
	} `json:"dependencies"`
}

// File is the jar to install for a version.
type File struct {
	URL, Filename, SHA1, SHA512 string
}

func (v Version) File() (File, bool) {
	if len(v.Files) == 0 {
		return File{}, false
	}
	pick := v.Files[0]
	for _, f := range v.Files {
		if f.Primary {
			pick = f
			break
		}
	}
	return File{URL: pick.URL, Filename: pick.Filename, SHA1: pick.Hashes.SHA1, SHA512: pick.Hashes.SHA512}, true
}

// RequiredProjects lists the other projects this version cannot run without.
func (v Version) RequiredProjects() []string {
	var ids []string
	for _, dep := range v.Dependencies {
		if dep.Type == "required" && dep.ProjectID != "" {
			ids = append(ids, dep.ProjectID)
		}
	}
	return ids
}

func jsonList(values []string) string {
	data, _ := json.Marshal(values)
	return string(data)
}

// Search finds plugins or mods that run on the given software and Minecraft version.
// Results are paged; total is how many projects match in all.
func Search(ctx context.Context, query string, software Software, gameVersion string, offset, limit int) (projects []Project, total int, err error) {
	loaderFacet := make([]string, len(software.Loaders))
	for i, loader := range software.Loaders {
		loaderFacet[i] = "categories:" + loader
	}
	facets := [][]string{{"project_type:" + software.ModrinthType}, loaderFacet, {"server_side:required", "server_side:optional"}}
	if gameVersion != "" {
		facets = append(facets, []string{"versions:" + gameVersion})
	}
	facetJSON, _ := json.Marshal(facets)

	// With nothing typed, show the most downloaded; otherwise the best matches.
	index := "relevance"
	if strings.TrimSpace(query) == "" {
		index = "downloads"
	}
	params := url.Values{
		"query": {query}, "index": {index}, "facets": {string(facetJSON)},
		"limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)},
	}
	var reply struct {
		Hits []struct {
			ProjectID   string `json:"project_id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Author      string `json:"author"`
			Downloads   int    `json:"downloads"`
			IconURL     string `json:"icon_url"`
		} `json:"hits"`
		Total int `json:"total_hits"`
	}
	if err := getJSON(ctx, modrinth+"/search?"+params.Encode(), nil, &reply); err != nil {
		return nil, 0, err
	}
	projects = make([]Project, len(reply.Hits))
	for i, hit := range reply.Hits {
		projects[i] = Project{ID: hit.ProjectID, Title: hit.Title, Description: hit.Description, Author: hit.Author, Downloads: hit.Downloads, IconURL: hit.IconURL, Source: SourceModrinth}
	}
	return projects, reply.Total, nil
}

// Latest returns the newest version of a project that runs on the given software and Minecraft version.
func Latest(ctx context.Context, projectID string, software Software, gameVersion string) (Version, bool, error) {
	params := url.Values{"loaders": {jsonList(software.Loaders)}}
	if gameVersion != "" {
		params.Set("game_versions", jsonList([]string{gameVersion}))
	}
	var versions []Version
	if err := getJSON(ctx, modrinth+"/project/"+url.PathEscape(projectID)+"/version?"+params.Encode(), nil, &versions); err != nil {
		return Version{}, false, err
	}
	if len(versions) == 0 {
		return Version{}, false, nil
	}
	for _, version := range versions {
		if version.VersionType == "release" {
			return version, true, nil
		}
	}
	return versions[0], true, nil
}

// Identify looks up files by their SHA-1 fingerprint. A file Modrinth recognises is
// exactly what its author published there.
func Identify(ctx context.Context, sha1s []string) (map[string]Version, error) {
	found := map[string]Version{}
	if len(sha1s) == 0 {
		return found, nil
	}
	body, _ := json.Marshal(map[string]any{"hashes": sha1s, "algorithm": "sha1"})
	err := getJSON(ctx, modrinth+"/version_files", bytes.NewReader(body), &found)
	return found, err
}

// Updates returns, for each recognised file, the newest full release that runs on this server.
// Test builds are never offered as updates.
func Updates(ctx context.Context, sha1s []string, software Software, gameVersion string) (map[string]Version, error) {
	found := map[string]Version{}
	if len(sha1s) == 0 {
		return found, nil
	}
	request := map[string]any{"hashes": sha1s, "algorithm": "sha1", "loaders": software.Loaders, "version_types": []string{"release"}}
	if gameVersion != "" {
		request["game_versions"] = []string{gameVersion}
	}
	body, _ := json.Marshal(request)
	err := getJSON(ctx, modrinth+"/version_files/update", bytes.NewReader(body), &found)
	return found, err
}

// Titles returns the display name and icon for a set of projects.
func Titles(ctx context.Context, ids []string) (map[string]Project, error) {
	out := map[string]Project{}
	if len(ids) == 0 {
		return out, nil
	}
	var reply []struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		IconURL string `json:"icon_url"`
	}
	if err := getJSON(ctx, modrinth+"/projects?ids="+url.QueryEscape(jsonList(ids)), nil, &reply); err != nil {
		return nil, err
	}
	for _, p := range reply {
		out[p.ID] = Project{ID: p.ID, Title: p.Title, IconURL: p.IconURL}
	}
	return out, nil
}
