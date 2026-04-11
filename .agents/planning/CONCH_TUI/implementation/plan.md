# TUI Facelift — Implementation Plan

## Goal

Upgrade the conch TUI to use `charm.land/lipgloss/v2` for a unified, styled look with:
- A persistent **top bar** showing the active tool name
- A persistent **bottom command bar** showing context-sensitive keybindings
- An **internal tab system** allowing multiple tools to be open simultaneously
- Shared rendering components reused across all views

## Current Architecture

- `internal/tui/root.go` — push/pop navigation stack (`Root` model)
- `internal/tui/menu.go` — main menu (plain text)
- `internal/tui/views.go` — view router + executeView + listView
- `internal/tui/burrow.go` — ticket/session management view
- `internal/tui/ticket.go` — repo picker / new ticket view
- `internal/tui/worktrees.go` — worktree management view
- `internal/tui/mantle.go` — agents/skills/docs/settings viewer
- `internal/tui/planning.go` — planning wizard
- `internal/tui/planning_sessions.go` — planning sessions list
- `internal/tui/wrap.go` — text wrap helper

All views render raw strings. No lipgloss usage. `charm.land/lipgloss/v2` is already in go.mod.

## Target Architecture

### New files

1. `internal/tui/theme.go` — lipgloss styles (colors, borders, text styles) as package-level vars
2. `internal/tui/chrome.go` — `RenderChrome(title, body, helpLine string, w, h int) string` — composes top bar + scrollable body + bottom bar
3. `internal/tui/tabs.go` — `tabsModel`: replaces `Root`; holds a slice of named tabs, each containing a `tea.Model`; handles tab switching (ctrl+t new tab, ctrl+w close, ctrl+left/right switch)

### Modified files

4. `internal/tui/root.go` — `Root` delegates to `tabsModel`; `New()` creates initial menu tab
5. `internal/tui/menu.go` — styled with lipgloss; uses `RenderChrome`
6. `internal/tui/views.go` — all views use `RenderChrome`; help lines extracted per-view
7. `internal/tui/burrow.go` — uses `RenderChrome`
8. `internal/tui/ticket.go` — uses `RenderChrome`
9. `internal/tui/worktrees.go` — uses `RenderChrome`
10. `internal/tui/mantle.go` — uses `RenderChrome`
11. `internal/tui/planning.go` — uses `RenderChrome`
12. `internal/tui/planning_sessions.go` — uses `RenderChrome`

## Tab System Design

- `tabsModel` holds `[]tab` where `tab = {name string; stack []tea.Model}`
- Active tab index tracked; each tab has its own push/pop stack
- Global keys: `ctrl+t` opens a new menu tab, `ctrl+w` closes current tab (quit if last), `ctrl+right`/`ctrl+left` cycle tabs
- Tab bar rendered inside the top chrome bar alongside the tool name

## Chrome Layout

```
┌─────────────────────────────────────────────────────┐
│  [menu] [burrow] [worktrees]          conch 🐌       │  ← top bar (tab bar + app name)
├─────────────────────────────────────────────────────┤
│                                                     │
│   <view body>                                       │
│                                                     │
├─────────────────────────────────────────────────────┤
│  ↑/↓ navigate  enter select  ctrl+t new tab  q quit │  ← bottom command bar
└─────────────────────────────────────────────────────┘
```

## Theme

- Background: terminal default
- Top/bottom bars: subtle background (`#1a1a2e` or adaptive dark)
- Active tab: bold + accent color (`#7c3aed` purple or `#06b6d4` cyan)
- Inactive tab: dimmed
- Cursor/selection: accent color highlight
- Borders: rounded lipgloss border on panels
- Status/error: red for errors, green for success

## Step Breakdown

### Step 1 — Foundation (theme + chrome + tabs)
- `theme.go`: define all styles
- `chrome.go`: `RenderChrome` function
- `tabs.go` + updated `root.go`: tab system replacing push/pop

### Step 2 — Menu + shared list component
- Restyle `menu.go` with lipgloss + chrome
- Extract reusable `renderList` helper in `chrome.go`

### Step 3 — Tool views
- Restyle `burrow.go`, `ticket.go`, `worktrees.go`, `views.go` (listView, executeView, stubView)

### Step 4 — Mantle + planning views
- Restyle `mantle.go`, `planning.go`, `planning_sessions.go`
