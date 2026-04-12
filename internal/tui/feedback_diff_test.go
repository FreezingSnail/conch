package tui

import (
	"strings"
	"testing"

	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
	tea "github.com/charmbracelet/bubbletea"
)

func baseDiffView() diffView {
	return diffView{
		ticket:   makeTicket(1, "My ticket"),
		commits:  []git.Commit{{Hash: "abc1234", Subject: "first commit"}},
		allNotes: make(map[string][]db.FeedbackNote),
		loaded:   true,
		w:        120,
		h:        40,
	}
}

func test_diff_view_state_commits(t *testing.T) {
	t.Helper()
	v := baseDiffView()
	v.commitFiles = []string{"main.go", "util.go"}
	out := v.View()
	if !strings.Contains(out, "Commits") {
		t.Errorf("expected 'Commits' header:\n%s", out)
	}
	if !strings.Contains(out, "Files Changed") {
		t.Errorf("expected 'Files Changed' header:\n%s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected file name in preview:\n%s", out)
	}
}

func test_diff_view_state_files(t *testing.T) {
	t.Helper()
	v := baseDiffView()
	v.state = stateFiles
	v.hunks = []git.DiffHunk{
		{FilePath: "main.go", HunkHeader: "@@ -1,3 +1,4 @@", Lines: []string{"+foo"}},
	}
	out := v.View()
	if !strings.Contains(out, "Files") {
		t.Errorf("expected 'Files' header:\n%s", out)
	}
	if !strings.Contains(out, "Diff") {
		t.Errorf("expected 'Diff' header:\n%s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected file name in list:\n%s", out)
	}
}

func test_hunk_note_marker(t *testing.T) {
	t.Helper()
	hunkWithNote := git.DiffHunk{FilePath: "foo.go", HunkHeader: "@@ -1,2 +1,3 @@"}
	hunkWithout := git.DiffHunk{FilePath: "bar.go", HunkHeader: "@@ -5,2 +5,3 @@"}
	notes := map[string][]db.FeedbackNote{
		hunkKey(hunkWithNote): {{ID: 1, TicketID: 1, Body: "looks good"}},
	}
	v := baseDiffView()
	v.state = stateFiles
	v.hunks = []git.DiffHunk{hunkWithNote, hunkWithout}
	v.allNotes = notes
	// foo.go (fileCur=0) has notes → notes panel visible
	out := v.View()
	if !strings.Contains(out, "Notes") {
		t.Errorf("expected Notes panel for selected file with notes:\n%s", out)
	}
	// bar.go (fileCur=1) has no notes → ● on non-selected file with notes
	v.fileCur = 1
	out = v.View()
	if !strings.Contains(out, "●") {
		t.Errorf("expected ● marker on non-selected file with notes:\n%s", out)
	}
}

func test_notes_panel_hidden_when_no_notes(t *testing.T) {
	t.Helper()
	v := baseDiffView()
	v.state = stateFiles
	v.hunks = []git.DiffHunk{{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@"}}
	out := v.View()
	if strings.Contains(out, "Notes") {
		t.Errorf("expected no Notes panel when no notes and not editing:\n%s", out)
	}
}

func test_notes_panel_visible_when_editing(t *testing.T) {
	t.Helper()
	v := baseDiffView()
	v.state = stateFiles
	v.hunks = []git.DiffHunk{{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@"}}
	m, _ := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	out := m.(diffView).View()
	if !strings.Contains(out, "Notes") {
		t.Errorf("expected Notes panel visible while editing:\n%s", out)
	}
}

func test_note_editor_open_close(t *testing.T) {
	t.Helper()
	v := baseDiffView()
	v.state = stateFiles
	v.hunks = []git.DiffHunk{{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@"}}
	m, _ := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got := m.(diffView)
	if !got.editing {
		t.Fatal("expected editing=true after pressing n")
	}
	m, _ = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = m.(diffView)
	if got.editing {
		t.Error("expected editing=false after esc")
	}
	if got.editText != "" || got.editNoteID != 0 {
		t.Errorf("expected editText/editNoteID cleared, got %q/%d", got.editText, got.editNoteID)
	}
}

func test_note_editor_text_input(t *testing.T) {
	t.Helper()
	v := diffView{ticket: makeTicket(1, "t"), allNotes: make(map[string][]db.FeedbackNote), editing: true}
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

func test_note_editor_save(t *testing.T) {
	t.Helper()
	v := baseDiffView()
	v.state = stateFiles
	v.hunks = []git.DiffHunk{{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@"}}
	v.editing = true
	v.editText = "looks good"
	m, cmd := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m.(diffView)
	if cmd == nil {
		t.Fatal("expected non-nil Cmd for create_feedback_note")
	}
	if got.editing {
		t.Error("expected editing=false after enter")
	}
}

func test_note_delete(t *testing.T) {
	t.Helper()
	hunk := git.DiffHunk{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@"}
	notes := map[string][]db.FeedbackNote{
		hunkKey(hunk): {{ID: 42, TicketID: 1, Body: "fix this"}},
	}
	v := baseDiffView()
	v.state = stateFiles
	v.hunks = []git.DiffHunk{hunk}
	v.allNotes = notes
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if cmd == nil {
		t.Fatal("expected non-nil Cmd for delete_feedback_note")
	}
}

func test_scroll_independent_of_file_cursor(t *testing.T) {
	t.Helper()
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "+line"
	}
	v := baseDiffView()
	v.state = stateFiles
	v.focus = focusRight
	v.h = 10
	v.hunks = []git.DiffHunk{{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@", Lines: lines}}
	m, _ := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	got := m.(diffView)
	if got.fileCur != 0 {
		t.Errorf("expected fileCur unchanged=0, got %d", got.fileCur)
	}
	if got.fileScroll != 1 {
		t.Errorf("expected fileScroll=1, got %d", got.fileScroll)
	}
}

func test_space_page_down(t *testing.T) {
	t.Helper()
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = "+line"
	}
	v := baseDiffView()
	v.state = stateFiles
	v.focus = focusRight
	v.h = 10
	v.hunks = []git.DiffHunk{{FilePath: "a.go", HunkHeader: "@@ -1 +1 @@", Lines: lines}}
	m, _ := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	got := m.(diffView)
	if got.fileScroll == 0 {
		t.Error("expected fileScroll > 0 after space")
	}
}

func TestDiffView_state_commits(t *testing.T)       { test_diff_view_state_commits(t) }
func TestDiffView_state_files(t *testing.T)         { test_diff_view_state_files(t) }
func TestDiffView_hunk_note_marker(t *testing.T)    { test_hunk_note_marker(t) }
func TestDiffView_notes_panel_hidden(t *testing.T)  { test_notes_panel_hidden_when_no_notes(t) }
func TestDiffView_notes_panel_visible(t *testing.T) { test_notes_panel_visible_when_editing(t) }
func TestNoteEditorOpenClose(t *testing.T)          { test_note_editor_open_close(t) }
func TestNoteEditorTextInput(t *testing.T)          { test_note_editor_text_input(t) }
func TestNoteEditorSave(t *testing.T)               { test_note_editor_save(t) }
func TestNoteDelete(t *testing.T)                   { test_note_delete(t) }
func TestDiffView_scroll_independent(t *testing.T)  { test_scroll_independent_of_file_cursor(t) }
func TestDiffView_space_page_down(t *testing.T)     { test_space_page_down(t) }
