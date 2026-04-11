# Task: Restyle mantle, planning, and planning sessions views

Update `internal/tui/mantle.go`, `internal/tui/planning.go`, and `internal/tui/planning_sessions.go` to implement `Titler` and `Helper` interfaces and use lipgloss styles.

## Requirements

### All views
- Add `Title() string` and `HelpLine() string`
- Remove inline help text from `View()` body
- Store `w, h int`; update from `tea.WindowSizeMsg`

### mantle.go
- Section tab bar: use `StyleActiveTab`/`StyleInactiveTab`
- List cursor: use `StyleCursor`
- Reader mode: use `StyleTitle` for the `"Mantle — <title>"` header
- `HelpLine()` returns context-sensitive hint based on whether reader mode is active

### planning.go
- Step indicator / progress: use `StyleTitle` for step headings
- Input prompts: use `StyleBody`
- Error/status: `StyleError`/`StyleSuccess`

### planning_sessions.go
- List cursor: use `StyleCursor`
- Status: `StyleError`/`StyleSuccess`
