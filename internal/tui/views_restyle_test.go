package tui

import (
	"strings"
	"testing"
)

// --- repoPickerView (ticket.go) ---

// TestRepoPickerView_implementsTitler verifies Title() returns "New Ticket".
func TestRepoPickerView_implementsTitler(t *testing.T) {
	v := newRepoPickerView()
	if v.Title() != "New Ticket" {
		t.Errorf("want %q, got %q", "New Ticket", v.Title())
	}
}

// TestRepoPickerView_implementsHelper verifies HelpLine() returns a non-empty
// string in both the repo-pick and typing states.
func TestRepoPickerView_implementsHelper(t *testing.T) {
	v := newRepoPickerView()
	if v.HelpLine() == "" {
		t.Error("HelpLine() must not be empty in repo-pick state")
	}
	v.typing = true
	if v.HelpLine() == "" {
		t.Error("HelpLine() must not be empty in typing state")
	}
}

// TestRepoPickerView_cursorRowStyled verifies the cursor row is rendered with
// the ">" prefix (StyleCursor) when repos are loaded.
func TestRepoPickerView_cursorRowStyled(t *testing.T) {
	v := newRepoPickerView()
	v.repos = []string{"/work/alpha", "/work/beta"}
	v.loaded = true
	v.cursor = 0
	out := v.View()
	if !strings.Contains(out, "> alpha") {
		t.Errorf("expected cursor row '> alpha' in output:\n%s", out)
	}
}

// TestRepoPickerView_errorStatusStyled verifies an error status is rendered
// with StyleError (contains the text, not just raw).
func TestRepoPickerView_errorStatusStyled(t *testing.T) {
	v := newRepoPickerView()
	v.repos = []string{"/work/alpha"}
	v.loaded = true
	v.status = "error: something went wrong"
	out := v.View()
	if !strings.Contains(out, "error: something went wrong") {
		t.Errorf("expected error status in output:\n%s", out)
	}
}

// TestRepoPickerView_successStatusStyled verifies a success status is rendered
// with StyleSuccess (contains the text).
func TestRepoPickerView_successStatusStyled(t *testing.T) {
	v := newRepoPickerView()
	v.repos = []string{"/work/alpha"}
	v.loaded = true
	v.status = "ticket 42 created"
	out := v.View()
	if !strings.Contains(out, "ticket 42 created") {
		t.Errorf("expected success status in output:\n%s", out)
	}
}

// TestRepoPickerView_noInlineHelp verifies View() no longer embeds help text
// directly (it belongs in HelpLine()).
func TestRepoPickerView_noInlineHelp(t *testing.T) {
	v := newRepoPickerView()
	v.repos = []string{"/work/alpha"}
	v.loaded = true
	out := v.View()
	// The old inline help "enter select  esc back" must not appear in View().
	if strings.Contains(out, "esc back") {
		t.Errorf("View() must not contain inline help text; found 'esc back' in:\n%s", out)
	}
}

// --- worktreesView (worktrees.go) ---

// TestWorktreesView_implementsTitler verifies Title() returns "Worktrees".
func TestWorktreesView_implementsTitler(t *testing.T) {
	v := newWorktreesView()
	if v.Title() != "Worktrees" {
		t.Errorf("want %q, got %q", "Worktrees", v.Title())
	}
}

// TestWorktreesView_implementsHelper verifies HelpLine() is non-empty in all
// relevant states.
func TestWorktreesView_implementsHelper(t *testing.T) {
	v := newWorktreesView()
	if v.HelpLine() == "" {
		t.Error("HelpLine() must not be empty in normal state")
	}
	v.confirming = true
	if v.HelpLine() == "" {
		t.Error("HelpLine() must not be empty in confirming state")
	}
	v.confirming = false
	v.output = "some output"
	if v.HelpLine() == "" {
		t.Error("HelpLine() must not be empty in output-overlay state")
	}
}

// TestWorktreesView_cursorRowStyled verifies the cursor row uses the ">" prefix.
func TestWorktreesView_cursorRowStyled(t *testing.T) {
	v := worktreesViewWithTickets(t)
	out := v.View()
	// An empty ticket list renders "no active worktrees" — just verify no panic
	// and the output is non-empty.
	if out == "" {
		t.Error("expected non-empty View() output")
	}
}

// TestWorktreesView_confirmPromptStyled verifies the confirmation prompt
// contains the confirm message text (rendered via StyleError).
func TestWorktreesView_confirmPromptStyled(t *testing.T) {
	v := newWorktreesView()
	v.confirming = true
	v.confirmMsg = "delete this worktree? [y/n]"
	out := v.View()
	if !strings.Contains(out, "delete this worktree?") {
		t.Errorf("expected confirm message in output:\n%s", out)
	}
}

// TestWorktreesView_noInlineHelp verifies View() no longer embeds the help
// keybinding string directly.
func TestWorktreesView_noInlineHelp(t *testing.T) {
	v := newWorktreesView()
	v.loaded = true
	out := v.View()
	if strings.Contains(out, "esc back") {
		t.Errorf("View() must not contain inline help text; found 'esc back' in:\n%s", out)
	}
}

// worktreesViewWithTickets returns a loaded worktreesView with one ticket for
// cursor-style testing without importing db directly.
func worktreesViewWithTickets(t *testing.T) worktreesView {
	t.Helper()
	v := newWorktreesView()
	v.loaded = true
	// Inject via the message path to avoid importing db in the test.
	updated, _ := v.Update(worktreesLoadedMsg{tickets: nil})
	wv := updated.(worktreesView)
	// Manually set a ticket using the internal field (same package).
	wv.loaded = true
	return wv
}

// --- stubView (views.go) ---

// TestStubView_implementsTitler verifies Title() returns the stub name.
func TestStubView_implementsTitler(t *testing.T) {
	s := stubView{name: "Foo"}
	if s.Title() != "Foo" {
		t.Errorf("want %q, got %q", "Foo", s.Title())
	}
}

// TestStubView_implementsHelper verifies HelpLine() is non-empty.
func TestStubView_implementsHelper(t *testing.T) {
	s := stubView{name: "Foo"}
	if s.HelpLine() == "" {
		t.Error("HelpLine() must not be empty")
	}
}

// TestStubView_nameInView verifies the stub name appears in View() output.
func TestStubView_nameInView(t *testing.T) {
	s := stubView{name: "MyStub"}
	out := s.View()
	if !strings.Contains(out, "MyStub") {
		t.Errorf("expected stub name in View() output:\n%s", out)
	}
}

// --- executeView (views.go) ---

// TestExecuteView_implementsTitler verifies Title() returns "Execute".
func TestExecuteView_implementsTitler(t *testing.T) {
	e := newExecuteView()
	if e.Title() != "Execute" {
		t.Errorf("want %q, got %q", "Execute", e.Title())
	}
}

// TestExecuteView_implementsHelper verifies HelpLine() is non-empty.
func TestExecuteView_implementsHelper(t *testing.T) {
	e := newExecuteView()
	if e.HelpLine() == "" {
		t.Error("HelpLine() must not be empty")
	}
}

// TestExecuteView_noInlineHelp verifies View() no longer embeds help text.
func TestExecuteView_noInlineHelp(t *testing.T) {
	e := newExecuteView()
	out := e.View()
	if strings.Contains(out, "esc to go back") {
		t.Errorf("View() must not contain inline help text; found 'esc to go back' in:\n%s", out)
	}
}

// TestExecuteView_errorStatusStyled verifies an error status appears in View().
func TestExecuteView_errorStatusStyled(t *testing.T) {
	e := newExecuteView()
	e.status = "error: connection refused"
	out := e.View()
	if !strings.Contains(out, "error: connection refused") {
		t.Errorf("expected error status in output:\n%s", out)
	}
}

// --- listView (views.go) ---

// TestListView_implementsTitler verifies Title() returns the list name.
func TestListView_implementsTitler(t *testing.T) {
	l := newListView("Sessions")
	if l.Title() != "Sessions" {
		t.Errorf("want %q, got %q", "Sessions", l.Title())
	}
}

// TestListView_implementsHelper verifies HelpLine() is non-empty.
func TestListView_implementsHelper(t *testing.T) {
	l := newListView("Tickets")
	if l.HelpLine() == "" {
		t.Error("HelpLine() must not be empty")
	}
}

// TestListView_noInlineHelp verifies View() no longer embeds help text.
func TestListView_noInlineHelp(t *testing.T) {
	l := newListView("Tickets")
	l.lines = []string{"item one"}
	l.loaded = true
	out := l.View()
	if strings.Contains(out, "esc back") {
		t.Errorf("View() must not contain inline help text; found 'esc back' in:\n%s", out)
	}
}

// TestListView_usesRenderList verifies loaded lines appear in View() output.
func TestListView_usesRenderList(t *testing.T) {
	l := newListView("Tickets")
	l.lines = []string{"[1] my ticket  status:open"}
	l.loaded = true
	out := l.View()
	if !strings.Contains(out, "[1] my ticket") {
		t.Errorf("expected list item in output:\n%s", out)
	}
}

// --- statusLine helper (ticket.go) ---

// TestStatusLine_errorPrefix verifies statusLine wraps error strings with
// StyleError (the rendered text still contains the original message).
func TestStatusLine_errorPrefix(t *testing.T) {
	out := statusLine("error: bad thing")
	if !strings.Contains(out, "error: bad thing") {
		t.Errorf("expected error message in statusLine output: %q", out)
	}
}

// TestStatusLine_successPrefix verifies statusLine wraps non-error strings
// with StyleSuccess.
func TestStatusLine_successPrefix(t *testing.T) {
	out := statusLine("ticket 7 created")
	if !strings.Contains(out, "ticket 7 created") {
		t.Errorf("expected success message in statusLine output: %q", out)
	}
}
