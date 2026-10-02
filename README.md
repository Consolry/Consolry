# Consolry

A game server panel that knows your game. Pre-release: this repository holds the first working slice of the panel and daemon, plus the website.

## What works today

- **Daemon** (`consolry-daemon`): runs servers as processes on Windows or Linux, keeps their console history, and stops them cleanly.
- **Panel** (`consolry`): first-run admin account, sign-in, nodes, servers, start/stop/kill, and a live console in the browser.
- One SQLite file for the panel's data. The web interface is built into the panel binary.

Not built yet: files, backups, schedules, roles, containers on Linux, and everything Minecraft-specific. See the [roadmap](https://www.consolry.com/roadmap).

## Layout

| Path | What it is |
| --- | --- |
| `cmd/consolry` | The panel program |
| `cmd/consolry-daemon` | The daemon program, one per machine that hosts servers |
| `internal/panel` | Panel API, database and embedded web interface |
| `internal/daemon` | Process management and the daemon API |
| `panel-ui` | The panel's React interface |
| `website` | The public site at consolry.com |

## Build and run

Needs Go 1.27 or newer and Node 20 or newer.

```sh
# 1. Build the web interface (it is embedded into the panel)
cd panel-ui
npm install
npm run build
cd ..

# 2. Build both programs into ./bin
go build -o bin/ ./cmd/...

# 3. Start the daemon, then the panel
./bin/consolry-daemon     # prints a token the first time it starts
./bin/consolry            # open http://127.0.0.1:8700
```

In the panel: create the admin account, add a node with the address `http://127.0.0.1:8750` and the daemon's token, then create a server.

A server's start command runs inside its own folder under the daemon's data directory (`daemon-data/servers/<id>` by default). Put the server files there before starting it.

## Tests

```sh
go test ./...
```

## Security notes for this early version

- The panel and daemon listen on `127.0.0.1` by default and speak plain HTTP. Do not expose either to the internet yet; put them behind a reverse proxy with HTTPS if you must.
- Servers run as normal processes with the daemon's own permissions. Only run software you trust.

## Licence

The core will be released under AGPL-3.0.
