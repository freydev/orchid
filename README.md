# orchid

Local web dashboard for running and supervising coding agents (Claude Code, Codex, plain shells) in tmux sessions across projects and git worktrees.

## Requirements

- Go 1.26+
- tmux
- git
- Node.js 24 + npm — only to rebuild the web frontend (the built bundle in `web/dist` is committed)
- Agents you want to run: `claude`, `codex` (optional)

## Quick start

```bash
git clone git@github.com:freydev/orchid.git
cd orchid
go build -o orchid ./cmd/orchid
./orchid serve            # foreground, http://127.0.0.1:8081
```

Open http://127.0.0.1:8081 (or run `./orchid open`).

Build the binary in the repo root: on start orchid picks up `plugins/orchid-status` next to the binary and loads it into Claude sessions, so their status (running / done / needs input) is shown on the dashboard. To use another location set `ORCHID_PLUGIN_DIR`.

## Running as a daemon

```bash
./orchid start [-host H -port P]   # start in background
./orchid status                    # daemon status and session count
./orchid stop                      # stop daemon
./orchid install-service           # start at login (launchd on macOS, systemd --user on Linux)
./orchid uninstall-service
```

## CLI

```
orchid serve [-host H -port P]   run the server in the foreground
orchid start [-host H -port P]   start the server as a background daemon
orchid stop                      stop the daemon
orchid status                    show daemon status and session count
orchid open                      open the dashboard in a browser
orchid ls                        list sessions
orchid create <agent> [cwd]      create a session (claude | codex | shell)
orchid rm <id>                   delete a session
orchid restart <id>              restart a stopped session
```

## Configuration

`~/.config/orchid/config.json` (or `$XDG_CONFIG_HOME/orchid/config.json`). All fields are optional and most can be changed from the Settings dialog in the UI. Defaults: host `127.0.0.1`, port `8081`, idle sessions are stopped after 120 minutes.

## Frontend development

The frontend is Svelte 5 + TypeScript in `web/src`, bundled by rollup into `web/dist`, which is embedded into the Go binary.

```bash
cd web
npm install
npm run build     # or: npm run watch
cd ..
go build -o orchid ./cmd/orchid   # rebuild to embed the new bundle
```

## Tests

```bash
go test ./...
```

⚠️ Do not run tests from inside an orchid session: `$TMUX` is set there, and tests will create (and may kill) sessions in your real tmux server. Run them from a plain terminal.
