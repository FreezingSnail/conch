package tui

import (
	"strings"
	"testing"
)

// TestRenderChrome_containsActiveTab verifies the active tab name appears in
// the output rendered with the active style (bold/accent).
func TestRenderChrome_containsActiveTab(t *testing.T) {
	tabs := []TabInfo{{Name: "Tasks"}, {Name: "Worktrees"}}
	out := RenderChrome(tabs, 0, "conch", "body content", "q quit", 80, 24)
	if !strings.Contains(out, "Tasks") {
		t.Error("expected active tab name 'Tasks' in output")
	}
}

// TestRenderChrome_containsAppName verifies the app name appears in the top bar.
func TestRenderChrome_containsAppName(t *testing.T) {
	tabs := []TabInfo{{Name: "Tasks"}}
	out := RenderChrome(tabs, 0, "conch", "body", "help", 80, 24)
	if !strings.Contains(out, "conch") {
		t.Error("expected app name 'conch' in output")
	}
}

// TestRenderChrome_containsHelpLine verifies the helpLine appears in the bottom bar.
func TestRenderChrome_containsHelpLine(t *testing.T) {
	tabs := []TabInfo{{Name: "Tasks"}}
	out := RenderChrome(tabs, 0, "conch", "body", "q quit", 80, 24)
	if !strings.Contains(out, "q quit") {
		t.Error("expected helpLine 'q quit' in output")
	}
}

// TestRenderChrome_containsBody verifies the body string appears in the output.
func TestRenderChrome_containsBody(t *testing.T) {
	tabs := []TabInfo{{Name: "Tasks"}}
	out := RenderChrome(tabs, 0, "conch", "my body text", "help", 80, 24)
	if !strings.Contains(out, "my body text") {
		t.Error("expected body text in output")
	}
}

// TestRenderList_cursorRow verifies the selected row is prefixed with "> ".
func TestRenderList_cursorRow(t *testing.T) {
	items := []string{"alpha", "beta", "gamma"}
	out := RenderList(items, 1, 40)
	if !strings.Contains(out, "> beta") {
		t.Errorf("expected cursor on 'beta', got:\n%s", out)
	}
}

// TestRenderList_nonCursorRow verifies non-selected rows are prefixed with "  ".
func TestRenderList_nonCursorRow(t *testing.T) {
	items := []string{"alpha", "beta"}
	out := RenderList(items, 0, 40)
	if !strings.Contains(out, "  beta") {
		t.Errorf("expected non-cursor row '  beta', got:\n%s", out)
	}
}

// TestRenderList_empty verifies RenderList handles an empty slice without panic.
func TestRenderList_empty(t *testing.T) {
	out := RenderList([]string{}, 0, 40)
	if out != "" {
		t.Errorf("expected empty string for empty list, got %q", out)
	}
}
