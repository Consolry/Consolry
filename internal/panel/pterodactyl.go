package panel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Consolry/Consolry/internal/minecraft"
)

// A one-time importer from Pterodactyl. It uses Pterodactyl's client API, the same one its
// own pages use, so a normal account key (starting "ptlc_") is enough: no admin access.

type pterodactyl struct {
	base string
	key  string
}

var pteroClient = &http.Client{Timeout: 30 * time.Minute}

func newPterodactyl(address, key string) (pterodactyl, error) {
	parsed, err := url.Parse(strings.TrimSpace(address))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return pterodactyl{}, errors.New("enter your Pterodactyl panel's address, such as https://panel.example.com")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return pterodactyl{}, errors.New("enter an API key from your Pterodactyl account")
	}
	return pterodactyl{base: parsed.Scheme + "://" + parsed.Host, key: key}, nil
}

func (p pterodactyl) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.base+"/api/client"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	res, err := pteroClient.Do(req)
	if err != nil {
		return errors.New("Pterodactyl could not be reached at that address")
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return errors.New("Pterodactyl did not accept that API key. Make one under Account, API Credentials")
	case res.StatusCode >= 300:
		var problem struct {
			Errors []struct {
				Detail string `json:"detail"`
			} `json:"errors"`
		}
		_ = json.NewDecoder(res.Body).Decode(&problem)
		if len(problem.Errors) > 0 && problem.Errors[0].Detail != "" {
			return errors.New("Pterodactyl said: " + problem.Errors[0].Detail)
		}
		return fmt.Errorf("Pterodactyl answered %s", res.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// pteroServer is what the importer needs to know about one Pterodactyl server.
type pteroServer struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	MemoryMB   int    `json:"memoryMb"`
	Egg        string `json:"egg"`
	// Software is "paper", "purpur" or "fabric" when Consolry recognises the egg, else "".
	Software    string `json:"software"`
	DockerImage string `json:"-"`
	Startup     string `json:"-"`
}

func (p pterodactyl) servers(ctx context.Context) ([]pteroServer, error) {
	var reply struct {
		Data []struct {
			Attributes struct {
				Identifier  string `json:"identifier"`
				Name        string `json:"name"`
				DockerImage string `json:"docker_image"`
				Invocation  string `json:"invocation"`
				Limits      struct {
					Memory int `json:"memory"`
				} `json:"limits"`
				Relationships struct {
					Egg struct {
						Attributes struct {
							Name string `json:"name"`
						} `json:"attributes"`
					} `json:"egg"`
				} `json:"relationships"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := p.do(ctx, http.MethodGet, "?include=egg&per_page=100", nil, &reply); err != nil {
		return nil, err
	}
	servers := []pteroServer{}
	for _, item := range reply.Data {
		server := pteroServer{
			Identifier: item.Attributes.Identifier, Name: item.Attributes.Name, MemoryMB: item.Attributes.Limits.Memory,
			Egg: item.Attributes.Relationships.Egg.Attributes.Name, DockerImage: item.Attributes.DockerImage, Startup: item.Attributes.Invocation,
		}
		server.Software = recogniseEgg(server.Egg + " " + server.Startup)
		servers = append(servers, server)
	}
	return servers, nil
}

func recogniseEgg(text string) string {
	text = strings.ToLower(text)
	for _, software := range []string{"purpur", "paper", "fabric"} {
		if strings.Contains(text, software) {
			return software
		}
	}
	return ""
}

var javaInImage = regexp.MustCompile(`java_?(\d+)`)
var versionInLog = regexp.MustCompile(`Starting minecraft server version (\d+\.\d+(?:\.\d+)?)`)

// variables reads a server's start-up settings, such as which jar it runs.
func (p pterodactyl) variables(ctx context.Context, id string) (map[string]string, error) {
	var reply struct {
		Data []struct {
			Attributes struct {
				Name  string `json:"env_variable"`
				Value string `json:"server_value"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := p.do(ctx, http.MethodGet, "/servers/"+id+"/startup", nil, &reply); err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, item := range reply.Data {
		values[item.Attributes.Name] = item.Attributes.Value
	}
	return values, nil
}

// archive packs a server's whole folder on Pterodactyl's side and returns a link to download it.
// The archive name is returned too, so it can be deleted afterwards.
func (p pterodactyl) archive(ctx context.Context, id string) (link, name string, err error) {
	var listing struct {
		Data []struct {
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := p.do(ctx, http.MethodGet, "/servers/"+id+"/files/list?directory=%2F", nil, &listing); err != nil {
		return "", "", err
	}
	files := []string{}
	for _, item := range listing.Data {
		files = append(files, item.Attributes.Name)
	}
	if len(files) == 0 {
		return "", "", errors.New("that server has no files")
	}
	var packed struct {
		Attributes struct {
			Name string `json:"name"`
		} `json:"attributes"`
	}
	if err := p.do(ctx, http.MethodPost, "/servers/"+id+"/files/compress", map[string]any{"root": "/", "files": files}, &packed); err != nil {
		return "", "", fmt.Errorf("could not pack the server's files: %w", err)
	}
	var signed struct {
		Attributes struct {
			URL string `json:"url"`
		} `json:"attributes"`
	}
	if err := p.do(ctx, http.MethodGet, "/servers/"+id+"/files/download?file="+url.QueryEscape("/"+packed.Attributes.Name), nil, &signed); err != nil {
		return "", packed.Attributes.Name, err
	}
	return signed.Attributes.URL, packed.Attributes.Name, nil
}

// --- the import page ---

func (a *App) handlePteroServers(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL string `json:"url"`
		Key string `json:"key"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	p, err := newPterodactyl(input.URL, input.Key)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	servers, err := p.servers(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, servers)
}

func (a *App) handlePteroImport(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL        string `json:"url"`
		Key        string `json:"key"`
		Identifier string `json:"identifier"`
		NodeID     int64  `json:"nodeId"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	p, err := newPterodactyl(input.URL, input.Key)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	node, err := a.store.Node(input.NodeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "choose a machine for the server")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Hour)
	defer cancel()

	servers, err := p.servers(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var source pteroServer
	for _, server := range servers {
		if server.Identifier == input.Identifier {
			source = server
		}
	}
	if source.Identifier == "" {
		writeError(w, http.StatusNotFound, "that server is not on your Pterodactyl account")
		return
	}
	variables, _ := p.variables(ctx, source.Identifier)

	// Make the server on this side first, empty, so its folder exists.
	id, err := newServerID(source.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	spec, row, notes := importedSpec(id, source, variables, node.ID)
	user, _ := r.Context().Value(userKey).(User)
	row.OwnerID = user.ID
	if err := node.call(ctx, http.MethodPost, "/servers", spec, nil); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	undo := func() { _ = node.call(context.WithoutCancel(ctx), http.MethodDelete, "/servers/"+id, nil, nil) }

	link, archiveName, err := p.archive(ctx, source.Identifier)
	if archiveName != "" {
		// The archive is only needed for the copy; leave Pterodactyl as it was.
		defer func() {
			cleanup, stop := context.WithTimeout(context.Background(), time.Minute)
			defer stop()
			_ = p.do(cleanup, http.MethodPost, "/servers/"+source.Identifier+"/files/delete", map[string]any{"root": "/", "files": []string{archiveName}}, nil)
		}()
	}
	if err != nil {
		undo()
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var copied struct {
		Files int `json:"files"`
	}
	if err := node.slow(ctx, http.MethodPost, "/servers/"+id+"/files/import", map[string]string{"url": link}, &copied); err != nil {
		undo()
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	// "latest" on Pterodactyl does not say which version; the server's own log does.
	if row.Kind == "minecraft" && (row.MCVersion == "" || row.MCVersion == "latest") {
		row.MCVersion = ""
		if match := versionInLog.FindSubmatch(readServerFile(ctx, node, id, "logs/latest.log")); match != nil {
			row.MCVersion = string(match[1])
		} else {
			notes = append(notes, "Consolry could not tell which Minecraft version this is, so plugin search may show plugins for other versions. Start the server once, then pick its version under Settings.")
		}
	}
	if spec.Java == 0 && row.MCVersion != "" {
		spec.Java = minecraft.JavaFor(ctx, row.MCVersion)
		_ = node.call(ctx, http.MethodPut, "/servers/"+id, spec, nil)
	}
	if spec.Java > 0 {
		if warning := ensureJava(ctx, node, spec.Java); warning != "" {
			notes = append(notes, warning)
		}
	}
	if err := a.store.CreateServer(row); err != nil {
		undo()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.log(r, id, fmt.Sprintf("Imported from Pterodactyl (%d files)", copied.Files))
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "files": copied.Files, "notes": notes})
}

// importedSpec works out how to start an imported server: as a Minecraft server Consolry
// understands when the egg is Paper, Purpur or Fabric, otherwise with Pterodactyl's own command.
func importedSpec(id string, source pteroServer, variables map[string]string, nodeID int64) (daemonSpec, ServerRow, []string) {
	notes := []string{}
	memory := source.MemoryMB
	if memory <= 0 {
		memory = 4096
		notes = append(notes, "The server had no memory limit on Pterodactyl, so it was given 4 GB. Change it on the Startup tab.")
	}
	row := ServerRow{ID: id, Name: source.Name, NodeID: nodeID, Kind: "generic"}
	jar := variables["SERVER_JARFILE"]
	if jar == "" {
		jar = "server.jar"
	}
	java := 0
	if match := javaInImage.FindStringSubmatch(source.DockerImage); match != nil {
		java, _ = strconv.Atoi(match[1])
	}

	if source.Software != "" || strings.Contains(strings.ToLower(source.Startup), "-jar") {
		size := strconv.Itoa(memory) + "M"
		args := []string{"-Xms" + size, "-Xmx" + size}
		args = append(args, ColourOptions...)
		spec := daemonSpec{ID: id, Command: "java", Args: append(args, "-jar", jar, "nogui"), StopCommand: "stop", Java: java}
		if source.Software != "" {
			row.Kind, row.Software = "minecraft", source.Software
			row.MCVersion = variables["MINECRAFT_VERSION"]
		} else {
			notes = append(notes, "Consolry did not recognise this egg as Paper, Purpur or Fabric, so it starts the jar but plugin tools are off.")
		}
		return spec, row, notes
	}

	// Anything else keeps Pterodactyl's command, with its placeholders filled in.
	command := source.Startup
	for name, value := range variables {
		command = strings.ReplaceAll(command, "{{"+name+"}}", value)
	}
	command = strings.ReplaceAll(command, "{{SERVER_MEMORY}}", strconv.Itoa(memory))
	command = strings.ReplaceAll(command, "{{SERVER_PORT}}", variables["SERVER_PORT"])
	executable, args := splitCommand(command)
	notes = append(notes, "This is not a Minecraft server Consolry knows, so it uses Pterodactyl's start command. Pterodactyl ran it inside Docker, so check the command works on this machine.")
	return daemonSpec{ID: id, Command: executable, Args: args}, row, notes
}
