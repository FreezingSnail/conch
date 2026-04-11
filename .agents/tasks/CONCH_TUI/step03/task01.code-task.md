# Task: Restyle burrow, ticket, worktrees, and generic list/execute views

Update `internal/tui/burrow.go`, `internal/tui/ticket.go`, `internal/tui/worktrees.go`, and `internal/tui/views.go` to implement `Titler` and `Helper` interfaces and use lipgloss styles.

## Requirements for each view

### All views
- Add `Title() string` returning the view's display name (e.g. `"Burrow"`, `"New Ticket"`, `"Worktrees"`, `"Sessions"`, `"Tickets"`, `"Execute"`)
- Add `HelpLine() string` returning the context-sensitive keybinding hint (move existing hint strings here)
- Remove inline help text from `View()` body
- Store `w, h int`; update from `tea.WindowSizeMsg`
- Apply `StyleError` to error status strings, `StyleSuccess` to success strings

### burrow.go
- Tab bar inside body: use `StyleActiveTab`/`StyleInactiveTab` for the Pending/Needs Review/Complete tabs
- Table header row: use `StyleTitle` (bold)
- Cursor row: use `StyleCursor`

### worktrees.go
- Cursor row: use `StyleCursor`
- Confirmation prompt: use `StyleError` color

### ticket.go (repoPickerView)
- Cursor row: use `StyleCursor`
- Status line: `StyleError` or `StyleSuccess` based on content

### views.go
- `listView.View()`: use `RenderList` helper
- `executeView.View()`: style the status line with `StyleError`/`StyleSuccess`
- `stubView.View()`: use `StyleTitle` for name
