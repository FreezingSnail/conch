package tui

import (
	"strings"
	"testing"

	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
	tea "github.com/charmbracelet/bubbletea"
)

// test_diff_view_render: given a diffView with mock commits and hunks, call
// View() and expect three panel columns to be present without panicking.
func test_diff_view_render(t *testing.T) {
	t.Helper()

	v := diffView{
		ticket:  makeTicket(1, "My ticket"),
		commits: []git.Commit{{Hash: "abc1234", Subject: "first commit"}},
		hunks: []git.DiffHunk{
			{FilePath: "main.go", HunkHeader: "@@ -1,3 +1,4 @@", Lines: []string{"+foo"}},
		},
		allNotes: make(map[string][]db.FeedbackNote),
		loaded:   true,
		w:        120,
		h:        40,
	}

	out := v.View()

	// All three panel headers must appear.
	if !strings.Contains(out, "Commits") {
		t.Errorf("expected 'Commits' panel header in output:\n%s", out)
	}
	if !strings.Contains(out, "Hunks") {
		t.Errorf("expected 'Hunks' panel header in output:\n%s", out)
	}
	if !strings.Contains(out, "Notes") {
		t.Errorf("expected 'Notes' panel header in output:\n%s", out)
	}
}

// test_hunk_note_marker: given allNotes with an entry for one hunk key, expect
// View() output contains ● for that hunk and not for hunks without notes.
func test_hunk_note_marker(t *testing.T) {
	t.Helper()

	hunkWithNote := git.DiffHunk{FilePath: "foo.go", HunkHeader: "@@ -1,2 +1,3 @@"}
	hunkWithout := git.DiffHunk{FilePath: "bar.go", HunkHeader: "@@ -5,2 +5,3 @@"}

	notes := map[string][]db.FeedbackNote{
		hunkKey(hunkWithNote): {
			{ID: 1, TicketID: 1, Body: "looks good"},
		},
	}

	v := diffView{
		ticket:   makeTicket(1, "ticket"),
		commits:  []git.Commit{{Hash: "deadbeef", Subject: "some commit"}},
		hunks:    []git.DiffHunk{hunkWithNote, hunkWithout},
		allNotes: notes,
		loaded:   true,
		w:        120,
		h:        40,
	}

	out := v.View()

	if !strings.Contains(out, "●") {
		t.Errorf("expected ● marker for hunk with notes, got:\n%s", out)
	}

	// Count occurrences: only one hunk has notes so ● should appear exactly once.
	count := strings.Count(out, "●")
	if count != 1 {
		t.Errorf("expected exactly 1 ● marker, got %d in:\n%s", count, out)
	}
}

// TestDiffView_render wraps the spec-named test so `go test` picks it up.
func TestDiffView_render(t *testing.T) { test_diff_view_render(t) }

// TestDiffView_hunk_note_marker wraps the spec-named test so `go test` picks it up.
func TestDiffView_hunk_note_marker(t *testing.T) { test_hunk_note_marker(t) }

// test_note_editor_open_close: pressing n from focusCenter opens the editor;
// pressing esc closes it and clears state.
func test_note_editor_open_close(t *testing.T) {
	t.Helper()

	v := diffView{
		ticket:   makeTicket(1, "ticket"),
		commits:  []git.Commit{{Hash: "abc", Subject: "c"}},
		hunks:    []git.DiffHunk{{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@"}},
		allNotes: make(map[string][]db.FeedbackNote),
		focus:    focusCenter,
		loaded:   true,
		w:        120,
		h:        40,
	}

	m, _ := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got := m.(diffView)
	if !got.editing {
		t.Fatal("expected editing=true after pressing n")
	}

	m, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = m.(diffView)
	if got.editing {
		t.Error("expected editing=false after pressing esc")
	}
	if got.editText != "" || got.editNoteID != 0 {
		t.Errorf("expected editText and editNoteID cleared, got %q / %d", got.editText, got.editNoteID)
	}
}

// test_note_editor_text_input: while editing, printable keys accumulate into
// editText and backspace removes the last character.
func test_note_editor_text_input(t *testing.T) {
	t.Helper()

	v := diffView{
		ticket:   makeTicket(1, "ticket"),
		allNotes: make(map[string][]db.FeedbackNote),
		editing:  true,
	}

	m, _ := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m, _ = m.(diffView).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	got := m.(diffView)
	if got.editText != "hi" {
		t.Errorf("expected editText=%q, got %q", "hi", got.editText)
	}

	m, _ = got.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got = m.(diffView)
	if got.editText != "h" {
		t.Errorf("expected editText=%q after backspace, got %q", "h", got.editText)
	}
}

// test_note_editor_save: pressing enter while editing with editNoteID==0 returns
// a non-nil Cmd (the create_feedback_note IPC call) and closes the editor.
func test_note_editor_save(t *testing.T) {
	t.Helper()

	v := diffView{
		ticket:   makeTicket(1, "ticket"),
		commits:  []git.Commit{{Hash: "abc", Subject: "c"}},
		hunks:    []git.DiffHunk{{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@"}},
		allNotes: make(map[string][]db.FeedbackNote),
		editing:  true,
		editText: "looks good",
		// editNoteID == 0 → create path
	}

	m, cmd := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m.(diffView)

	if cmd == nil {
		t.Fatal("expected non-nil Cmd for create_feedback_note, got nil")
	}
	if got.editing {
		t.Error("expected editing=false after enter")
	}
}

// test_note_delete: pressing d with focusRight and a note present returns a
// non-nil Cmd (the delete_feedback_note IPC call).
func test_note_delete(t *testing.T) {
	t.Helper()

	hunk := git.DiffHunk{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@"}
	notes := map[string][]db.FeedbackNote{
		hunkKey(hunk): {{ID: 42, TicketID: 1, Body: "fix this"}},
	}

	v := diffView{
		ticket:   makeTicket(1, "ticket"),
		commits:  []git.Commit{{Hash: "abc", Subject: "c"}},
		hunks:    []git.DiffHunk{hunk},
		allNotes: notes,
		focus:    focusRight,
		loaded:   true,
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if cmd == nil {
		t.Fatal("expected non-nil Cmd for delete_feedback_note, got nil")
	}
}

// TestNoteEditorOpenClose wraps the spec-named test so `go test` picks it up.
func TestNoteEditorOpenClose(t *testing.T) { test_note_editor_open_close(t) }

// TestNoteEditorTextInput wraps the spec-named test so `go test` picks it up.
func TestNoteEditorTextInput(t *testing.T) { test_note_editor_text_input(t) }

// TestNoteEditorSave wraps the spec-named test so `go test` picks it up.
func TestNoteEditorSave(t *testing.T) { test_note_editor_save(t) }

// TestNoteDelete wraps the spec-named test so `go test` picks it up.
func TestNoteDelete(t *testing.T) { test_note_delete(t) }
