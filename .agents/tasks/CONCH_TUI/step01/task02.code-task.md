# Task: Implement RenderChrome helper

Create `internal/tui/chrome.go` with the `RenderChrome` function that composes the top bar, body, and bottom command bar into a full-screen view string.

## Requirements

- Signature: `func RenderChrome(tabs []TabInfo, activeTab int, toolName, body, helpLine string, w, h int) string`
- `TabInfo` is a struct `{ Name string }` defined in this file
- Top bar layout (full width `w`):
  - Left side: tab pills rendered with `StyleActiveTab` / `StyleInactiveTab`
  - Right side: app name `"conch 🐌"` right-aligned using `lipgloss.PlaceHorizontal`
  - Background: `StyleTopBar`
- Body: the raw `body` string, height = `h - 4` (2 for top bar + 1 for bottom bar + 1 padding)
- Bottom bar (full width `w`):
  - `helpLine` rendered with `StyleBottomBar`
- Use `lipgloss.Width` to measure rendered tab strings for right-alignment math
- Export `TabInfo` and `RenderChrome`

## Also add

A helper `func RenderList(items []string, cursor int, w int) string` that renders a list with `StyleCursor` on the selected row. Used by menu and other list views.

## File

`internal/tui/chrome.go`
