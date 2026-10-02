// Package minecraft knows where Minecraft server software and plugins come from,
// and how to read a Minecraft server's log.
package minecraft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const userAgent = "Consolry (github.com/DinoNaedYT/Consolry)"

var client = &http.Client{Timeout: 20 * time.Second}

// getJSON fetches a URL and decodes its JSON reply. A body, if given, is sent as a JSON POST.
func getJSON(ctx context.Context, address string, body io.Reader, out any) error {
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		host := address
		if parsed, err := url.Parse(address); err == nil {
			host = parsed.Host
		}
		return fmt.Errorf("%s answered %s", host, res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// Software is a kind of Minecraft server.
type Software struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	About        string   `json:"about"`
	Folder       string   `json:"folder"`  // where plugins or mods go
	Loaders      []string `json:"loaders"` // Modrinth loader names it can run
	ModrinthType string   `json:"-"`
}

var softwares = []Software{
	{ID: "paper", Name: "Paper", About: "Plugins, fast. The usual choice.", Folder: "plugins", Loaders: []string{"paper", "spigot", "bukkit"}, ModrinthType: "plugin"},
	{ID: "purpur", Name: "Purpur", About: "Paper with extra settings.", Folder: "plugins", Loaders: []string{"purpur", "paper", "spigot", "bukkit"}, ModrinthType: "plugin"},
	{ID: "fabric", Name: "Fabric", About: "Lightweight mods.", Folder: "mods", Loaders: []string{"fabric"}, ModrinthType: "mod"},
}

func AllSoftware() []Software { return softwares }

func FindSoftware(id string) (Software, bool) {
	for _, s := range softwares {
		if s.ID == id {
			return s, true
		}
	}
	return Software{}, false
}

// Download is one server jar and what it needs.
type Download struct {
	URL     string
	SHA256  string
	JavaMin int
}

// isRelease reports whether a version string is a full release rather than a snapshot or release candidate.
func isRelease(version string) bool {
	for _, part := range strings.Split(version, ".") {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return version != ""
}

// newer reports whether version a is newer than b, comparing each dotted number.
func newer(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func releasesNewestFirst(versions []string) []string {
	out := []string{}
	for _, v := range versions {
		if isRelease(v) {
			out = append(out, v)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return newer(out[i], out[j]) })
	return out
}

// Versions lists the Minecraft releases a kind of server is available for, newest first.
func Versions(ctx context.Context, software string) ([]string, error) {
	switch software {
	case "paper":
		var reply struct {
			Versions map[string][]string `json:"versions"`
		}
		if err := getJSON(ctx, "https://fill.papermc.io/v3/projects/paper", nil, &reply); err != nil {
			return nil, err
		}
		var all []string
		for _, group := range reply.Versions {
			all = append(all, group...)
		}
		return releasesNewestFirst(all), nil
	case "purpur":
		var reply struct {
			Versions []string `json:"versions"`
		}
		if err := getJSON(ctx, "https://api.purpurmc.org/v2/purpur", nil, &reply); err != nil {
			return nil, err
		}
		return releasesNewestFirst(reply.Versions), nil
	case "fabric":
		var reply []struct {
			Version string `json:"version"`
			Stable  bool   `json:"stable"`
		}
		if err := getJSON(ctx, "https://meta.fabricmc.net/v2/versions/game", nil, &reply); err != nil {
			return nil, err
		}
		var all []string
		for _, v := range reply {
			if v.Stable {
				all = append(all, v.Version)
			}
		}
		return releasesNewestFirst(all), nil
	}
	return nil, errors.New("unknown server software")
}

// javaFor returns the oldest Java that can run a Minecraft version. Paper publishes this;
// when it has no entry for the version, fall back to the known cut-offs.
func javaFor(ctx context.Context, version string) int {
	var reply struct {
		Version struct {
			Java struct {
				Version struct {
					Minimum int `json:"minimum"`
				} `json:"version"`
			} `json:"java"`
		} `json:"version"`
	}
	if getJSON(ctx, "https://fill.papermc.io/v3/projects/paper/versions/"+url.PathEscape(version), nil, &reply) == nil && reply.Version.Java.Version.Minimum > 0 {
		return reply.Version.Java.Version.Minimum
	}
	switch {
	case !newer("1.17", version):
		if !newer("1.20.5", version) {
			return 21
		}
		if !newer("1.18", version) {
			return 17
		}
		return 16
	default:
		return 8
	}
}

// RecommendedFlags returns the Java options Paper suggests for a Minecraft version, or none if it publishes none.
func RecommendedFlags(ctx context.Context, version string) []string {
	var reply struct {
		Version struct {
			Java struct {
				Flags struct {
					Recommended []string `json:"recommended"`
				} `json:"flags"`
			} `json:"java"`
		} `json:"version"`
	}
	if !isRelease(version) || getJSON(ctx, "https://fill.papermc.io/v3/projects/paper/versions/"+url.PathEscape(version), nil, &reply) != nil {
		return []string{}
	}
	if reply.Version.Java.Flags.Recommended == nil {
		return []string{}
	}
	return reply.Version.Java.Flags.Recommended
}

// Resolve finds the newest build of a server for one Minecraft version.
func Resolve(ctx context.Context, software, version string) (Download, error) {
	if !isRelease(version) {
		return Download{}, errors.New("unknown Minecraft version")
	}
	switch software {
	case "paper":
		var build struct {
			Downloads map[string]struct {
				URL       string `json:"url"`
				Checksums struct {
					SHA256 string `json:"sha256"`
				} `json:"checksums"`
			} `json:"downloads"`
		}
		if err := getJSON(ctx, "https://fill.papermc.io/v3/projects/paper/versions/"+version+"/builds/latest", nil, &build); err != nil {
			return Download{}, err
		}
		file, ok := build.Downloads["server:default"]
		if !ok || file.URL == "" {
			return Download{}, errors.New("Paper has no download for that version")
		}
		return Download{URL: file.URL, SHA256: file.Checksums.SHA256, JavaMin: javaFor(ctx, version)}, nil
	case "purpur":
		return Download{URL: "https://api.purpurmc.org/v2/purpur/" + version + "/latest/download", JavaMin: javaFor(ctx, version)}, nil
	case "fabric":
		var loaders, installers []struct {
			Version string `json:"version"`
			Stable  bool   `json:"stable"`
		}
		if err := getJSON(ctx, "https://meta.fabricmc.net/v2/versions/loader", nil, &loaders); err != nil {
			return Download{}, err
		}
		if err := getJSON(ctx, "https://meta.fabricmc.net/v2/versions/installer", nil, &installers); err != nil {
			return Download{}, err
		}
		pick := func(list []struct {
			Version string `json:"version"`
			Stable  bool   `json:"stable"`
		}) string {
			for _, item := range list {
				if item.Stable {
					return item.Version
				}
			}
			if len(list) > 0 {
				return list[0].Version
			}
			return ""
		}
		loader, installer := pick(loaders), pick(installers)
		if loader == "" || installer == "" {
			return Download{}, errors.New("Fabric has no loader available")
		}
		return Download{
			URL:     "https://meta.fabricmc.net/v2/versions/loader/" + version + "/" + loader + "/" + installer + "/server/jar",
			JavaMin: javaFor(ctx, version),
		}, nil
	}
	return Download{}, errors.New("unknown server software")
}
