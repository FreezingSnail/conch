# conch

Autonomous development tool. Terminal-native, daemon-backed, LLM-powered via CLI harnesses (kiro, copilot).

## Architecture

```
cmd/conch/      TUI client
cmd/conchd/     Detached daemon (Unix socket + JSON)
internal/db/    SQLite brain (~/.conch/conch.db)
internal/daemon/ Socket server + request routing
internal/tui/   BubbleTea views (menu, plan, execute, tickets, sessions)
internal/harness/ CLI harness abstraction (kiro, copilot)
internal/client/ Daemon client helper
```

## Build

```sh
go build -o conch ./cmd/conch
go build -o conchd ./cmd/conchd
```

## Run

```sh
# start the daemon
./conch daemon start

# launch the TUI
./conch
```

## TUI Navigation

- `↑/↓` or `j/k` — navigate
- `enter` — select
- `esc` — go back
- `q` — quit (from main menu)

## Views

- **Plan** — hands terminal to kiro interactively, records session on exit
- **Execute** — sends a prompt to the daemon, kiro runs in background, status tracked in SQLite
- **Tickets** — lists tickets from DB (stub, rich system coming)
- **Sessions** — lists all sessions with status and timestamps

## Data

SQLite at `~/.conch/conch.db`. Tables: `tickets`, `tasks`, `sessions`, `session_logs`.

## Credits

- Slug mode adapted from [caveman](https://github.com/JuliusBrussee/caveman)
