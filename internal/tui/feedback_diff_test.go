package tui

import (
	"strings"
	"testing"

	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
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
