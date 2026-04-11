package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestMenu_Title verifies Title() returns "conch".
func TestMenu_Title(t *testing.T) {
	m := newMenu()
	if m.Title() != "conch" {
		t.Errorf("want %q, got %q", "conch", m.Title())
	}
}

// TestMenu_HelpLine verifies HelpLine() returns the expected navigation hint.
func TestMenu_HelpLine(t *testing.T) {
	m := newMenu()
	want := "↑/↓ navigate  enter select  shortcut jump  ctrl+t new tab  q quit"
	if m.HelpLine() != want {
		t.Errorf("want %q, got %q", want, m.HelpLine())
	}
}

// TestMenu_View_containsHeader verifies the styled header is present in View().
func TestMenu_View_containsHeader(t *testing.T) {
	m := newMenu()
	v := m.View()
	if !strings.Contains(v, "conch 🐌") {
		t.Errorf("expected 'conch 🐌' in View(), got:\n%s", v)
	}
}

// TestMenu_View_noInlineHelp verifies the inline help line is absent from View().
func TestMenu_View_noInlineHelp(t *testing.T) {
	m := newMenu()
	v := m.View()
	if strings.Contains(v, "↑/↓ navigate") {
		t.Errorf("inline help line should not appear in View(), got:\n%s", v)
	}
}

// TestMenu_View_containsItems verifies all menu items appear in View().
func TestMenu_View_containsItems(t *testing.T) {
	m := newMenu()
	v := m.View()
	for _, item := range menuItems {
		if !strings.Contains(v, item.label) {
			t.Errorf("expected item %q in View()", item.label)
		}
	}
}

// TestMenu_WindowSizeMsg_storesDimensions verifies w and h are stored on resize.
func TestMenu_WindowSizeMsg_storesDimensions(t *testing.T) {
	m := newMenu()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got := updated.(menu)
	if got.w != 100 || got.h != 30 {
		t.Errorf("want w=100 h=30, got w=%d h=%d", got.w, got.h)
	}
}

// TestMenu_implementsTitler verifies menu satisfies the Titler interface.
func TestMenu_implementsTitler(t *testing.T) {
	var _ Titler = newMenu()
}

// TestMenu_implementsHelper verifies menu satisfies the Helper interface.
func TestMenu_implementsHelper(t *testing.T) {
	var _ Helper = newMenu()
}
