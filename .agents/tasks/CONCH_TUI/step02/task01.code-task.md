# Task: Restyle menu view

Update `internal/tui/menu.go` to use lipgloss styles and the shared `RenderChrome` infrastructure.

## Requirements

- Add `Title() string` returning `"conch"` and `HelpLine() string` returning `"↑/↓ navigate  enter select  ctrl+t new tab  q quit"` to `menu`
- `View()`: replace raw string building with:
  - Use `RenderList(menuItems, m.cursor, m.w)` for the item list
  - Return just the body string (chrome is applied by `tabsModel.View()`)
  - Store `w, h int` on `menu` and update from `tea.WindowSizeMsg`
- Style the `"conch 🐌"` header line with `StyleTitle`
- Remove the inline help line from `View()` (it moves to `HelpLine()`)
