# Task: Define lipgloss theme package

Create `internal/tui/theme.go` with package-level lipgloss style variables used by all views.

## Requirements

- Import `charm.land/lipgloss/v2`
- Define the following exported style vars:
  - `StyleTopBar` — full-width bar, subtle dark background (`AdaptiveColor{Light:"#e2e8f0", Dark:"#1e1e2e"}`), bold text
  - `StyleBottomBar` — same background as top bar, dimmed text
  - `StyleActiveTab` — bold, accent foreground (`AdaptiveColor{Light:"#7c3aed", Dark:"#a78bfa"}`)
  - `StyleInactiveTab` — dimmed foreground
  - `StyleCursor` — accent foreground, bold (used for `>` cursor in lists)
  - `StyleTitle` — bold white/dark text
  - `StyleError` — red foreground
  - `StyleSuccess` — green foreground
  - `StyleBorder` — `lipgloss.RoundedBorder()`, border color matching accent
  - `StyleBody` — no special styling (pass-through, used for content area)
- No functions in this file, only `var` declarations

## File

`internal/tui/theme.go`
