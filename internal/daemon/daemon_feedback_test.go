package daemon

import (
	"testing"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/kiro"
)

// openTestDB opens an in-memory-equivalent test DB with HOME set to a temp dir.
func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	d, err := db.Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// seedTicket creates a ticket with no worktree (so git sync is a no-op).
func seedTicket(t *testing.T, database *db.DB) int64 {
	t.Helper()
	id, err := database.CreateTicket("T-1", "test ticket", "", "")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// test_daemon_feedback_notes_crud exercises create → list → update → delete via dispatch.
func test_daemon_feedback_notes_crud(t *testing.T) {
	database := openTestDB(t)
	ticketID := seedTicket(t, database)

	// create_feedback_note
	resp := dispatch(client.Request{
		Action:     "create_feedback_note",
		TicketID:   ticketID,
		CommitHash: "abc123",
		FilePath:   "main.go",
		HunkHeader: "@@ -1,3 +1,4 @@",
		NoteBody:   "original body",
	}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("create_feedback_note: %s", resp.Error)
	}
	noteID := resp.ID
	if noteID == 0 {
		t.Fatal("expected non-zero note ID")
	}

	// list_feedback_notes — expect 1 note with original body
	resp = dispatch(client.Request{
		Action:   "list_feedback_notes",
		TicketID: ticketID,
	}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("list_feedback_notes: %s", resp.Error)
	}
	if len(resp.FeedbackNotes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(resp.FeedbackNotes))
	}
	if resp.FeedbackNotes[0].Body != "original body" {
		t.Fatalf("unexpected body: %q", resp.FeedbackNotes[0].Body)
	}

	// update_feedback_note
	resp = dispatch(client.Request{
		Action:     "update_feedback_note",
		TicketID:   ticketID,
		CommitHash: "abc123",
		NoteID:     noteID,
		NoteBody:   "updated body",
	}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("update_feedback_note: %s", resp.Error)
	}

	// list again — expect updated body
	resp = dispatch(client.Request{
		Action:   "list_feedback_notes",
		TicketID: ticketID,
	}, database, kiro.Kiro{})
	if resp.FeedbackNotes[0].Body != "updated body" {
		t.Fatalf("expected updated body, got %q", resp.FeedbackNotes[0].Body)
	}

	// delete_feedback_note
	resp = dispatch(client.Request{
		Action:     "delete_feedback_note",
		TicketID:   ticketID,
		CommitHash: "abc123",
		NoteID:     noteID,
	}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("delete_feedback_note: %s", resp.Error)
	}

	// list again — expect 0 notes
	resp = dispatch(client.Request{
		Action:   "list_feedback_notes",
		TicketID: ticketID,
	}, database, kiro.Kiro{})
	if len(resp.FeedbackNotes) != 0 {
		t.Fatalf("expected 0 notes after delete, got %d", len(resp.FeedbackNotes))
	}
}

// test_daemon_mark_notes_addressed creates two notes via dispatch, marks them
// addressed, and verifies all notes have Addressed=true.
func test_daemon_mark_notes_addressed(t *testing.T) {
	database := openTestDB(t)
	ticketID := seedTicket(t, database)

	for _, body := range []string{"note one", "note two"} {
		resp := dispatch(client.Request{
			Action:     "create_feedback_note",
			TicketID:   ticketID,
			CommitHash: "abc123",
			FilePath:   "main.go",
			HunkHeader: "@@ -1 @@",
			NoteBody:   body,
		}, database, kiro.Kiro{})
		if !resp.OK {
			t.Fatalf("create_feedback_note: %s", resp.Error)
		}
	}

	// mark_notes_addressed
	resp := dispatch(client.Request{
		Action:   "mark_notes_addressed",
		TicketID: ticketID,
	}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("mark_notes_addressed: %s", resp.Error)
	}

	// list — all notes must have Addressed=true
	resp = dispatch(client.Request{
		Action:   "list_feedback_notes",
		TicketID: ticketID,
	}, database, kiro.Kiro{})
	if len(resp.FeedbackNotes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(resp.FeedbackNotes))
	}
	for _, n := range resp.FeedbackNotes {
		if !n.Addressed {
			t.Fatalf("expected note %d to be addressed", n.ID)
		}
	}
}

// TestDaemonFeedbackNotesCRUD is the Go test entry point for test_daemon_feedback_notes_crud.
func TestDaemonFeedbackNotesCRUD(t *testing.T) {
	test_daemon_feedback_notes_crud(t)
}

// TestDaemonMarkNotesAddressed is the Go test entry point for test_daemon_mark_notes_addressed.
func TestDaemonMarkNotesAddressed(t *testing.T) {
	test_daemon_mark_notes_addressed(t)
}

// test_replan_ticket_marks_addressed seeds notes, calls replan_ticket with TMUX
// set, and verifies all notes for that ticket are addressed in the DB.
// SpawnTmuxWindow will fail (no real tmux process), but MarkNotesAddressed runs
// before the spawn, so the DB state is the observable outcome under test.
func test_replan_ticket_marks_addressed(t *testing.T) {
	database := openTestDB(t)
	ticketID := seedTicket(t, database)

	// Seed two unaddressed notes.
	for _, body := range []string{"first note", "second note"} {
		_, err := database.CreateFeedbackNote(ticketID, "abc123", "main.go", "@@ -1 @@", body)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set TMUX so InTmux() returns true; SpawnTmuxWindow will fail but that is
	// after MarkNotesAddressed, which is the behaviour under test.
	t.Setenv("TMUX", "/tmp/tmux-test,0,0")

	dispatch(client.Request{Action: "replan_ticket", TicketID: ticketID}, database, kiro.Kiro{})

	// All notes must be addressed regardless of the tmux spawn outcome.
	notes, err := database.ListFeedbackNotesByTicket(ticketID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	for _, n := range notes {
		if !n.Addressed {
			t.Fatalf("expected note %d to be addressed", n.ID)
		}
	}
}

// TestReplanTicketMarksAddressed is the Go test entry point for test_replan_ticket_marks_addressed.
func TestReplanTicketMarksAddressed(t *testing.T) {
	test_replan_ticket_marks_addressed(t)
}
