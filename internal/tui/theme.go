// Package tui defines the visual theme for all views via shared lipgloss style
// variables. Centralising styles here ensures visual consistency and makes
// colour/layout changes a single-file edit.
//
// Colours are expressed as hex strings passed to lipgloss.Color. The dark-theme
// values from the original spec are used as the defaults; swap them for the
// light variants if a light-background renderer is needed.
package tui

import "charm.land/lipgloss/v2"

// barBg is the subtle dark background shared by the top and bottom bars.
var barBg = lipgloss.Color("#1e1e2e")

// accent is the purple foreground used for active tabs, cursors, and borders.
var accent = lipgloss.Color("#a78bfa")

// StyleTopBar is a full-width bar with a subtle dark background and bold text,
// used for the application header.
var StyleTopBar = lipgloss.NewStyle().
	Background(barBg).
	Bold(true)

// StyleBottomBar matches the top bar background with dimmed text, used for
// status/help lines at the bottom of the screen.
var StyleBottomBar = lipgloss.NewStyle().
	Background(barBg).
	Faint(true)

// StyleActiveTab renders the currently selected tab with bold accent colour.
var StyleActiveTab = lipgloss.NewStyle().
	Foreground(accent).
	Bold(true)

// StyleInactiveTab renders non-selected tabs with dimmed foreground.
var StyleInactiveTab = lipgloss.NewStyle().
	Faint(true)

// StyleCursor renders the `>` list cursor with accent colour and bold weight.
var StyleCursor = lipgloss.NewStyle().
	Foreground(accent).
	Bold(true)

// StyleTitle renders section titles in bold.
var StyleTitle = lipgloss.NewStyle().
	Bold(true)

// StyleError renders error messages in red.
var StyleError = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#ff5555"))

// StyleSuccess renders success messages in green.
var StyleSuccess = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#50fa7b"))

// StyleBorder wraps content with a rounded border coloured to match the accent.
var StyleBorder = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(accent)

// StyleBody is a pass-through style for content areas; it applies no decoration
// so callers can compose it with width/height constraints without visual side effects.
var StyleBody = lipgloss.NewStyle()
