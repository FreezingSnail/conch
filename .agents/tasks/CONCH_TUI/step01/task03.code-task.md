# Task: Implement internal tab system

Replace the push/pop `Root` model with a tab-aware model. Each tab has its own push/pop navigation stack. The user can open a new menu tab at any time with `ctrl+t`.

## Requirements

### `internal/tui/tabs.go` (new file)

Define:

```go
type tab struct {
    name  string
    stack []tea.Model
}

type tabsModel struct {
    tabs   []tab
    active int
    w, h   int
}
```

- `newTabsModel() tabsModel` — creates one tab named `"menu"` with `newMenu()` on its stack
- `(t tabsModel) Init() tea.Cmd` — calls `Init()` on the active tab's top model
- `(t tabsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)`:
  - `tea.WindowSizeMsg` — store `w`, `h`; forward to active top model
  - `ctrl+c` → `tea.Quit`
  - `ctrl+t` → append new tab with `newMenu()`, set active to new tab index
  - `ctrl+w` → close active tab; if last tab, `tea.Quit`; else activate previous
  - `ctrl+right` → cycle active tab forward
  - `ctrl+left` → cycle active tab backward
  - `pushMsg` → push onto active tab's stack
  - `popMsg` → pop active tab's stack; if stack becomes empty, close tab (same as ctrl+w)
  - All other msgs → forward to active tab's top model, update stack
- `(t tabsModel) View() string`:
  - Build `[]TabInfo` from tab names
  - Get `toolName` from active top model's type name (use a `Titler` interface: `Title() string`; fall back to `"conch"`)
  - Get `helpLine` from active top model (use `Helper` interface: `HelpLine() string`; fall back to `"ctrl+t new tab  ctrl+w close  ctrl+←/→ switch"`)
  - Call `RenderChrome(tabs, t.active, toolName, body, helpLine, t.w, t.h)`

### `internal/tui/root.go` (update)

- `Root` struct now just wraps `tabsModel`
- `New()` returns `Root{tabs: newTabsModel()}`
- Delegate `Init`, `Update`, `View` to `t.tabs`

## Interfaces (define in tabs.go)

```go
type Titler interface { Title() string }
type Helper interface { HelpLine() string }
```

## Tab naming

When a new menu tab is opened, name it `"menu"`. When a view is pushed onto a tab's stack, update the tab name to the pushed model's `Title()` (if it implements `Titler`). When popped back to menu, reset name to `"menu"`.
