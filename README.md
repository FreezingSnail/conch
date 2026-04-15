# conch

Autonomous development tool. Terminal-native, daemon-backed, LLM-powered via CLI harnesses (kiro, copilot).

---

## Quickstart

### How conch works

conch is made of two binaries:

- **`conchd`** — the daemon. Runs in the background, owns the SQLite database, handles all git operations, and launches kiro executor sessions. It listens on a Unix socket (`~/.conch/conch.sock`) and speaks JSON-RPC. You start it once and leave it running.
- **`conch`** — the TUI client. Connects to the daemon over the socket. All user interaction happens here. It is stateless; the daemon holds all state.

### Worktree-based workflow

conch is built around git worktrees. Every ticket gets its own worktree — an isolated checkout of your repo at a separate path — so multiple tasks can run in parallel without interfering with each other. The daemon creates and manages these worktrees automatically.

**Repo requirements:**

- The repo must have at least one commit. Git cannot create a worktree from an empty repo. If starting fresh, make an initial commit first:
  ```sh
  git init my-project && cd my-project
  git commit --allow-empty -m "init"
  ```
- For existing repos, any git repo with at least one commit works as-is.

### Config setup

Before using conch, tell it where your repos live. Edit `~/.conch/config.json`:

```json
{
  "work_dirs": ["/path/to/your/repo", "/path/to/another/repo"],
  "slug_mode": "slugineer"
}
```

- **`work_dirs`** — list of parent directories conch scans one level deep for git repos. For example, if your repos live in `~/code/`, add `~/code/` and conch will find all git repos inside it. You can also manage this from **Mantle → Settings** (`a` to add, `d` to delete) — no manual file editing required. The config file and `~/.conch/` directory are created automatically on first save.
- **`slug_mode`** — controls LLM verbosity: `slugineer` (default, compressed), `lite`, or `off`.

### Build and run

```sh
# 1. Build both binaries
go build -o conch ./cmd/conch
go build -o conchd ./cmd/conchd

# 2. Start the daemon (runs in background)
./conch daemon start

# 3. Launch the TUI
./conch
```

### Menu flow

The main menu is your entry point to all tools:

1. **Plan** — start here for new work. Wizard walks you through ticket number, title, idea, and repo selection. Hands off to kiro for planning; creates a worktree and ticket in the DB.
2. **Burrow** — execution dashboard. After planning, come here to start kiro executor sessions, monitor progress, and handle anything that needs human input.
3. **Tickets / Sessions** — read-only views of everything in the DB. Good for orientation.
4. **Worktrees** — inspect and prune worktrees attached to tickets.
5. **Feedback / Review** — manage feedback notes and PR review lifecycle after execution.
6. **Mantle** — docs, settings, agents, and skills browser. Press `?` from any view to jump directly to that view's docs section.

Navigate with `↑/↓` or `j/k`. Press `enter` to select, `esc` to go back, `q` to quit from the main menu.

---

## Architecture

```
cmd/conch/           TUI client + CLI entry point
cmd/conchd/          Detached daemon (Unix socket + JSON-RPC)
internal/db/         SQLite brain (~/.conch/conch.db)
internal/daemon/     Socket server, request routing, background runners
internal/tui/        BubbleTea views (menu, plan, burrow, tickets, sessions, mantle…)
internal/harness/    CLI harness abstraction (kiro, copilot)
internal/kiro/       Kiro-specific session + prompt helpers
internal/client/     Daemon client helper (send/receive JSON over Unix socket)
internal/git/        Git operations (worktrees, diffs, PR helpers)
internal/config/     Config load/save (~/.conch/config.json)
internal/assets/     Embedded agents, skills, README
internal/review/     PR review helpers
```

---

## Build

```sh
go build -o conch ./cmd/conch
go build -o conchd ./cmd/conchd
```

---

## Plan

Multi-step wizard for creating a new planning session with kiro.

**Flow:**
1. Enter a ticket number (free text, e.g. `42` or `PROJ-42`)
2. Enter a short title
3. Enter a rough idea (multiline; `ctrl+d` to confirm)
4. Pick one or more repos from the list
5. conch calls `plan_setup` on the daemon → creates ticket + worktree(s) in DB
6. Hands terminal to kiro (interactive) with a pre-built prompt
7. On kiro exit, calls `plan_complete` → records session; shows task summary

**Keybindings:**

| Key | Action |
|-----|--------|
| `enter` | Next step |
| `ctrl+d` | Confirm multiline idea input |
| `space` | Toggle repo selection |
| `esc` | Back |

**Daemon actions:** `list_repos`, `plan_setup`, `plan_complete`, `list_tasks`

---

## Burrow

Task execution dashboard. Lists tickets grouped by execution state; lets you start, monitor, and resume kiro executor sessions.

**Tabs:**
- **Pending** — tickets not yet started or currently running
- **Needs Review** — executor session completed, awaiting human review
- **Human Help** — session errored or flagged `needs_human`; resume here

**Keybindings:**

| Key | Action |
|-----|--------|
| `tab` | Switch tabs |
| `↑/↓` or `j/k` | Navigate tickets |
| `enter` | Start executor session (Pending) / Resume session (Human Help) |
| `l` | View session log for selected ticket |
| `D` | Delete ticket (with confirmation) |
| `r` | Refresh |
| `esc` | Back |

**Daemon actions:** `list_tickets`, `list_sessions`, `list_session_logs`, `list_tasks`, `execute`, `delete_ticket`

---

## Tickets

Read-only list of all tickets in the DB with their status.

**Columns:** `[id] title  status`

**Keybindings:**

| Key | Action |
|-----|--------|
| `r` | Refresh |
| `esc` | Back |

**Daemon actions:** `list_tickets`

---

## New Ticket

Create a ticket manually (without the full planning wizard). Pick a repo then enter a title.

**Flow:**
1. Select a repo from the list
2. Type a ticket title, press `enter`
3. Daemon creates the ticket; returns to menu

**Keybindings:**

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate repos |
| `enter` | Select repo / confirm title |
| `esc` | Back |

**Daemon actions:** `list_repos`, `create_ticket`

---

## Worktrees

Manage git worktrees associated with tickets. Shows all tickets that have worktrees; lets you prune or open them.

**Keybindings:**

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate |
| `enter` | Open worktree in shell |
| `p` | Prune worktree (with confirmation) |
| `r` | Refresh |
| `esc` | Back |

**Daemon actions:** `list_tickets`, `list_worktrees`, `prune_worktree`

---

## Sessions

Read-only list of all sessions (planning + executor) with harness, status, and timestamps.

**Columns:** `[id] harness  status  started`

**Keybindings:**

| Key | Action |
|-----|--------|
| `r` | Refresh |
| `esc` | Back |

**Daemon actions:** `list_sessions`

---

## Planning Sessions

Lists planning sessions specifically. Lets you resume an interrupted planning session by re-handing the terminal to kiro.

**Keybindings:**

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate |
| `enter` | Resume selected session |
| `r` | Refresh |
| `esc` | Back |

**Daemon actions:** `list_sessions`, `list_tickets`

---

## Mantle

In-app documentation and configuration browser. Four sections navigable via `tab`:

- **Agents** — browse embedded agent definitions (JSON); `enter` to read full prompt + metadata
- **Docs** — renders this README via glamour; `enter` to open reader
- **Settings** — view and edit `~/.conch/config.json`; add/remove work dirs, cycle slug mode
- **Skills** — browse embedded skill files; `enter` to read

**Reader keybindings (any section):**

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Scroll |
| `/` | Open search bar |
| `n` / `N` | Next / previous match |
| `esc` | Close reader / exit search |

**Settings keybindings:**

| Key | Action |
|-----|--------|
| `a` | Add work dir |
| `d` | Delete selected work dir |
| `s` | Cycle slug mode (`slugineer` → `lite` → `off`) |
| `w` | Toggle wenyan mode (`on` / `off`) |

---

## Feedback

Review and manage feedback notes attached to tickets. Two tabs:

- **Active** — tickets with unaddressed notes, or no notes yet
- **Archived** — tickets where all notes are addressed

**Keybindings:**

| Key | Action |
|-----|--------|
| `tab` | Switch tabs |
| `↑/↓` | Navigate tickets |
| `enter` | View notes for selected ticket |
| `a` | Mark selected note as addressed |
| `r` | Refresh |
| `esc` | Back |

**Daemon actions:** `list_tickets`, `list_feedback_notes`, `address_feedback_note`

---

## Review

PR review workflow. Four tabs tracking pull request lifecycle:

- **Open PRs** — newly created PRs
- **In Review** — PRs actively being reviewed
- **Ready for Approval** — PRs with review comments resolved; shows comment table on `enter`
- **Completed** — merged/closed PRs (read-only)

**Keybindings:**

| Key | Action |
|-----|--------|
| `tab` | Switch tabs |
| `↑/↓` | Navigate PRs |
| `enter` | View comments (Ready tab) / advance status |
| `r` | Refresh |
| `esc` | Back |

**Daemon actions:** `list_prs`, `list_pr_comments`, `update_pr_status`

---

## Config

Config file: `~/.conch/config.json`

```json
{
  "work_dirs": ["/path/to/repo1", "/path/to/repo2"],
  "slug_mode": "slugineer",
  "wenyan_mode": true
}
```

| Field | Description |
|-------|-------------|
| `work_dirs` | Parent directories conch scans one level deep for git repos (e.g. `~/code/`) |
| `slug_mode` | LLM response verbosity: `slugineer` (default), `lite`, `off` |
| `wenyan_mode` | When `true`, injects wenyan-ultra internal thinking mode into all agents. Cuts reasoning token usage ~80%. Default: `false` |

Edit via **Mantle → Settings** or directly in the file. Changes take effect on next load.

---

## Data

SQLite at `~/.conch/conch.db`.

| Table | Contents |
|-------|----------|
| `tickets` | Work items with title, status, repo |
| `tasks` | Sub-tasks linked to tickets |
| `sessions` | Planning + executor sessions with harness, status, timestamps |
| `session_logs` | Line-by-line log output from executor sessions |
| `worktrees` | Git worktree paths linked to tickets |
| `pull_requests` | PR metadata linked to tickets |
| `pr_review_comments` | Review comments linked to PRs |
| `feedback_notes` | Human feedback notes linked to tickets |

---

## Credits

- Slug mode adapted from [caveman](https://github.com/JuliusBrussee/caveman)
