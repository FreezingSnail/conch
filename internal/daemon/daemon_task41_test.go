package daemon

import (
	"testing"

	"github.com/FreezingSnail/conch/internal/client"
)

// TestDispatch_openPR_noWorktree creates a ticket with no worktree and asserts
// that open_pr returns an error rather than panicking or calling gh.
func TestDispatch_openPR_noWorktree(t *testing.T) {
	database := openTestDB(t)
	ticketID := seedTicket(t, database)

	resp := dispatch(client.Request{Action: "open_pr", TicketID: ticketID}, database)
	if resp.OK {
		t.Fatal("expected error response, got OK=true")
	}
	if resp.Error != "no worktree for ticket" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}

// TestDispatch_createTicket_noRepo creates a ticket with title only (no repo)
// and asserts OK=true with a non-zero ID.
func TestDispatch_createTicket_noRepo(t *testing.T) {
	database := openTestDB(t)

	resp := dispatch(client.Request{Action: "create_ticket", Title: "my ticket"}, database)
	if !resp.OK {
		t.Fatalf("create_ticket: %s", resp.Error)
	}
	if resp.ID == 0 {
		t.Fatal("expected non-zero ticket ID")
	}
}

// TestDispatch_deleteTicket_cascades creates a ticket, a task, and a feedback
// note via dispatch, deletes the ticket, then asserts list_tasks and
// list_feedback_notes return empty results.
func TestDispatch_deleteTicket_cascades(t *testing.T) {
	database := openTestDB(t)

	// Create ticket.
	resp := dispatch(client.Request{Action: "create_ticket", Title: "cascade ticket"}, database)
	if !resp.OK {
		t.Fatalf("create_ticket: %s", resp.Error)
	}
	ticketID := resp.ID

	// Create task.
	resp = dispatch(client.Request{Action: "create_task", TicketID: ticketID, Title: "a task"}, database)
	if !resp.OK {
		t.Fatalf("create_task: %s", resp.Error)
	}

	// Create feedback note.
	resp = dispatch(client.Request{
		Action:     "create_feedback_note",
		TicketID:   ticketID,
		CommitHash: "abc123",
		FilePath:   "main.go",
		HunkHeader: "@@ -1 @@",
		NoteBody:   "a note",
	}, database)
	if !resp.OK {
		t.Fatalf("create_feedback_note: %s", resp.Error)
	}

	// Delete ticket.
	resp = dispatch(client.Request{Action: "delete_ticket", TicketID: ticketID}, database)
	if !resp.OK {
		t.Fatalf("delete_ticket: %s", resp.Error)
	}

	// list_tasks must return empty.
	resp = dispatch(client.Request{Action: "list_tasks", TicketID: ticketID}, database)
	if !resp.OK {
		t.Fatalf("list_tasks: %s", resp.Error)
	}
	if len(resp.Tasks) != 0 {
		t.Fatalf("expected 0 tasks after delete, got %d", len(resp.Tasks))
	}

	// list_feedback_notes must return empty.
	resp = dispatch(client.Request{Action: "list_feedback_notes", TicketID: ticketID}, database)
	if !resp.OK {
		t.Fatalf("list_feedback_notes: %s", resp.Error)
	}
	if len(resp.FeedbackNotes) != 0 {
		t.Fatalf("expected 0 notes after delete, got %d", len(resp.FeedbackNotes))
	}
}
