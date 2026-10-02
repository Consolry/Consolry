# Consolry

A game server panel that knows your game. Pre-release: this repository holds the first working slice of the panel and daemon, plus the website.

## What works today

Consolry is for running game servers on your own Windows or Linux machine. You install it, you run it.

- **One program.** `consolry` is the panel and also runs the servers on the machine it is installed on. Nothing else to start, no token to copy.
- **Minecraft servers.** Pick Paper, Purpur or Fabric and a version. The server is downloaded and checked, and a suitable Java is installed privately if the machine has none.
- **Console, files and backups.** A live searchable console; a file manager with an editor; backups you can restore, download or delete.
- **Plugins and mods.** Search Modrinth, install with required dependencies, update, and see which installed files match what Modrinth published.
- **Players.** Who is online, plus the whitelist, operators and bans.
- **Schedules.** Restart, back up or run a console command daily, weekly or every few hours.
- **Automatic safety backups** before plugin changes and version switches, with old ones cleared out.
- **Settings.** Rename, change memory, switch Minecraft version, and edit the common game settings in a form that shows what will change.
- **Runs in the background.** On Windows it sits in the notification area with a menu to open the panel, start at sign-in, or quit. On Linux, `consolry -autostart on` sets it up as a service. Quitting saves and closes every server first.
- **Port forwarding.** One button asks your router to open a server's port, and tells you if your internet provider makes that impossible.
- **A coloured console** that keeps the server's own colours and marks each plugin green, yellow or red.
- **Crash explainer.** Reads the last run's log and explains common failures in plain language, on your machine.

Not built yet: two-factor login, extra users, notifications, Hangar and CurseForge sources, off-site backups, containers on Linux, an installer, and config forms for individual plugins. See the [roadmap](https://www.consolry.com/roadmap).

## Layout

| Path | What it is |
| --- | --- |
| `cmd/consolry` | The panel program |
| `cmd/consolry-daemon` | The daemon program, one per machine that hosts servers |
| `internal/panel` | Panel API, database and embedded web interface |
| `internal/daemon` | Process management, files, backups and the daemon API |
| `internal/minecraft` | Server downloads, Modrinth and the crash rules |
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

# 3. Start it
./bin/consolry            # then open http://127.0.0.1:8700
```

Create the admin account, then create a server. This machine is already set up as the place servers run.

Server files, backups and any Java that Consolry installs live in `daemon-data` next to where you start it; the panel's own data is `consolry.db`.

To run servers on a second machine as well, start `consolry-daemon` there and add it on the Nodes page with the token it prints.

## Tests

```sh
go test ./...
```

## Security notes for this early version

- The panel and daemon listen on `127.0.0.1` by default and speak plain HTTP. Do not expose either to the internet yet; put them behind a reverse proxy with HTTPS if you must.
- Servers run as normal processes with the daemon's own permissions. Only run software you trust.

## Licence

The core will be released under AGPL-3.0.
