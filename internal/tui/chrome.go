// Package tui — chrome.go composes the persistent top bar, scrollable body,
// and bottom help bar into a single full-screen string for every view.
package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// TabInfo describes a single tab pill in the top bar.
type TabInfo struct {
	Name string
}

// RenderChrome assembles the full-screen TUI frame:
//
//	top bar  — tab pills (left) + app name (right), styled with StyleTopBar
//	body     — raw content, height = h-4
//	bottom   — helpLine styled with StyleBottomBar
//
// w and h are the current terminal dimensions.
func RenderChrome(tabs []TabInfo, activeTab int, toolName, body, helpLine string, w, h int) string {
	// --- top bar ---
	// Render each tab pill with the appropriate style.
	var tabParts []string
	for i, t := range tabs {
		if i == activeTab {
			tabParts = append(tabParts, StyleActiveTab.Render(" "+t.Name+" "))
		} else {
			tabParts = append(tabParts, StyleInactiveTab.Render(" "+t.Name+" "))
		}
	}
	tabsStr := strings.Join(tabParts, "")

	appName := "conch 🐌"
	// Right-align the app name: fill the remaining width after the tab pills.
	rightWidth := w - lipgloss.Width(tabsStr)
	rightStr := lipgloss.PlaceHorizontal(rightWidth, lipgloss.Right, appName)

	topBar := StyleTopBar.Width(w).Render(tabsStr + rightStr)

	// --- body ---
	bodyHeight := h - 4 // 2 top-bar lines + 1 bottom bar + 1 padding
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	bodyStr := StyleBody.Width(w).Height(bodyHeight).Render(body)

	// --- bottom bar ---
	bottomBar := StyleBottomBar.Width(w).Render(helpLine)

	return strings.Join([]string{topBar, bodyStr, bottomBar}, "\n")
}

// RenderList renders a vertical list of items, highlighting the row at cursor
// with StyleCursor. Used by menu and other list views.
func RenderList(items []string, cursor int, w int) string {
	var b strings.Builder
	for i, item := range items {
		if i == cursor {
			b.WriteString(StyleCursor.Width(w).Render("> " + item))
		} else {
			b.WriteString(StyleBody.Width(w).Render("  " + item))
		}
		if i < len(items)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
