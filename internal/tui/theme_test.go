package tui

import (
	"testing"

	"charm.land/lipgloss/v2"
)

// TestThemeStylesAreDefined verifies that every exported style variable is
// initialised to a non-zero lipgloss.Style. A zero Style renders identically
// to StyleBody, so any accidental omission would be caught here.
func TestThemeStylesAreDefined(t *testing.T) {
	zero := lipgloss.NewStyle()

	cases := []struct {
		name  string
		style lipgloss.Style
	}{
		{"StyleTopBar", StyleTopBar},
		{"StyleBottomBar", StyleBottomBar},
		{"StyleActiveTab", StyleActiveTab},
		{"StyleInactiveTab", StyleInactiveTab},
		{"StyleCursor", StyleCursor},
		{"StyleTitle", StyleTitle},
		{"StyleError", StyleError},
		{"StyleSuccess", StyleSuccess},
		{"StyleBorder", StyleBorder},
	}

	for _, tc := range cases {
		if tc.style.String() == zero.String() {
			t.Errorf("%s has no styling (equals zero Style)", tc.name)
		}
	}
}

// TestStyleBodyIsPassThrough confirms StyleBody applies no decoration so it
// can be used as a neutral wrapper without unintended visual side effects.
func TestStyleBodyIsPassThrough(t *testing.T) {
	const input = "hello"
	if got := StyleBody.Render(input); got != input {
		t.Errorf("StyleBody.Render(%q) = %q, want %q", input, got, input)
	}
}
