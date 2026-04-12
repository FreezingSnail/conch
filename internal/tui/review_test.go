package tui

import (
	"strings"
	"testing"

	"github.com/FreezingSnail/conch/internal/db"
)

// TestReviewView_emptyState verifies that View() on a loaded zero-value reviewView
// does not panic and renders all four tab names.
func TestReviewView_emptyState(t *testing.T) {
	v := reviewView{loaded: true}
	out := v.View()
	for _, name := range reviewTabNames {
		if !strings.Contains(out, name) {
			t.Errorf("expected tab name %q in output", name)
		}
	}
}

// TestReviewView_openTab verifies that two open PRs appear in the View() output.
func TestReviewView_openTab(t *testing.T) {
	v := reviewView{
		loaded: true,
		tab:    reviewTabOpen,
		prs: []db.PullRequest{
			{ID: 1, PRNumber: 101, Title: "Fix the thing", Status: "open", Repo: "acme/repo", Author: "alice"},
			{ID: 2, PRNumber: 102, Title: "Add feature X", Status: "open", Repo: "acme/repo", Author: "bob"},
		},
	}
	out := v.View()
	if !strings.Contains(out, "Fix the thing") {
		t.Error("expected first PR title in output")
	}
	if !strings.Contains(out, "Add feature X") {
		t.Error("expected second PR title in output")
	}
}

// TestReviewView_readyTab_commentTable verifies that the comment table renders
// [✓] exactly once when one comment is approved and one is not.
func TestReviewView_readyTab_commentTable(t *testing.T) {
	v := reviewView{
		loaded:          true,
		tab:             reviewTabReady,
		viewingComments: true,
		comments: []db.PRReviewComment{
			{ID: 1, PRID: 10, Type: "suggestion", FilePath: "main.go", Line: 42, Body: "Use a constant here", Approved: true},
			{ID: 2, PRID: 10, Type: "nitpick", FilePath: "util.go", Line: 7, Body: "Rename this variable", Approved: false},
		},
	}
	out := v.View()
	count := strings.Count(out, "[✓]")
	// Header contains [✓] once as a column label, plus one for the approved comment.
	// We expect exactly 2 occurrences: header + 1 approved row.
	if count != 2 {
		t.Errorf("expected [✓] to appear 2 times (header + 1 approved row), got %d\noutput:\n%s", count, out)
	}
}
