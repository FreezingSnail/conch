package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/FreezingSnail/conch/internal/db"
)

// makeTicket is a helper that builds a minimal db.Ticket for tests.
func makeTicket(id int64, title string) db.Ticket {
	return db.Ticket{ID: id, Title: title, WorktreePath: "/fake/worktree", CreatedAt: time.Now()}
}

// makeNote is a helper that builds a db.FeedbackNote with the given addressed state.
func makeNote(id, ticketID int64, addressed bool) db.FeedbackNote {
	return db.FeedbackNote{ID: id, TicketID: ticketID, Addressed: addressed}
}

// test_feedback_view_render: given empty and populated feedbackView, call View(),
// expect no panic and tab names + ticket titles present in output.
func test_feedback_view_render(t *testing.T) {
	t.Helper()

	// Empty view (not yet loaded) — must not panic.
	empty := newFeedbackView()
	out := empty.View()
	if !strings.Contains(out, "Active") {
		t.Errorf("empty view: expected 'Active' tab name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Archived") {
		t.Errorf("empty view: expected 'Archived' tab name in output, got:\n%s", out)
	}

	// Populated view.
	tickets := []db.Ticket{
		makeTicket(1, "Alpha ticket"),
		makeTicket(2, "Beta ticket"),
	}
	notes := map[int64][]db.FeedbackNote{
		1: {makeNote(10, 1, false)}, // unaddressed → Active
		2: {makeNote(20, 2, true)},  // addressed   → Archived
	}
	v := feedbackView{
		tickets:       tickets,
		notesByTicket: notes,
		loaded:        true,
	}
	out = v.View()
	if !strings.Contains(out, "Active") {
		t.Errorf("populated view: expected 'Active' in output")
	}
	if !strings.Contains(out, "Archived") {
		t.Errorf("populated view: expected 'Archived' in output")
	}
	if !strings.Contains(out, "Alpha ticket") {
		t.Errorf("populated view: expected 'Alpha ticket' in output, got:\n%s", out)
	}
}

// test_feedback_view_tab_filter: given tickets with all-addressed notes and tickets
// with unaddressed notes, expect Active tab shows only unaddressed tickets,
// Archived tab shows only all-addressed tickets.
func test_feedback_view_tab_filter(t *testing.T) {
	t.Helper()

	tickets := []db.Ticket{
		makeTicket(1, "Unaddressed ticket"), // has unaddressed note → Active
		makeTicket(2, "Addressed ticket"),   // all notes addressed  → Archived
		makeTicket(3, "No notes ticket"),    // no notes             → Active
	}
	notes := map[int64][]db.FeedbackNote{
		1: {makeNote(10, 1, false)},
		2: {makeNote(20, 2, true)},
		// ticket 3 has no notes
	}
	v := feedbackView{
		tickets:       tickets,
		notesByTicket: notes,
		loaded:        true,
		tab:           feedbackTabActive,
	}

	// Active tab should contain tickets 1 and 3, not ticket 2.
	active := v.tabTickets()
	if len(active) != 2 {
		t.Fatalf("Active tab: expected 2 tickets, got %d", len(active))
	}
	for _, t2 := range active {
		if t2.ID == 2 {
			t.Errorf("Active tab: ticket 2 (all-addressed) should not appear")
		}
	}

	// Archived tab should contain only ticket 2.
	v.tab = feedbackTabArchived
	archived := v.tabTickets()
	if len(archived) != 1 {
		t.Fatalf("Archived tab: expected 1 ticket, got %d", len(archived))
	}
	if archived[0].ID != 2 {
		t.Errorf("Archived tab: expected ticket 2, got %d", archived[0].ID)
	}
}

// TestFeedbackView_render wraps the spec-named test so `go test` picks it up.
func TestFeedbackView_render(t *testing.T) { test_feedback_view_render(t) }

// TestFeedbackView_tab_filter wraps the spec-named test so `go test` picks it up.
func TestFeedbackView_tab_filter(t *testing.T) { test_feedback_view_tab_filter(t) }
