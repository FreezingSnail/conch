package daemon

import (
	"encoding/json"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
	"github.com/FreezingSnail/conch/internal/harness"
)

// handleFeedback routes feedback note actions. Returns (resp, true) for known
// actions, (zero, false) otherwise.
func handleFeedback(req client.Request, database *db.DB, _ harness.Harness) (client.Response, bool) {
	switch req.Action {
	case "list_feedback_notes":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		notes, err := database.ListFeedbackNotesByTicket(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, FeedbackNotes: notes}, true

	case "create_feedback_note":
		if req.TicketID == 0 || req.CommitHash == "" {
			return client.Response{Error: "ticket_id and commit_hash required"}, true
		}
		id, err := database.CreateFeedbackNote(req.TicketID, req.CommitHash, req.FilePath, req.HunkHeader, req.NoteBody)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		syncGitNote(database, req.TicketID, req.CommitHash)
		return client.Response{OK: true, ID: id}, true

	case "update_feedback_note":
		if req.NoteID == 0 {
			return client.Response{Error: "note_id required"}, true
		}
		existing, err := database.GetFeedbackNote(req.NoteID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		if err := database.UpdateFeedbackNote(req.NoteID, req.NoteBody); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		syncGitNote(database, existing.TicketID, existing.CommitHash)
		return client.Response{OK: true}, true

	case "delete_feedback_note":
		if req.NoteID == 0 {
			return client.Response{Error: "note_id required"}, true
		}
		existing, err := database.GetFeedbackNote(req.NoteID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		if err := database.DeleteFeedbackNote(req.NoteID); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		syncGitNote(database, existing.TicketID, existing.CommitHash)
		return client.Response{OK: true}, true

	case "mark_notes_addressed":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		if err := database.MarkNotesAddressed(req.TicketID); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	default:
		return client.Response{}, false
	}
}

// syncGitNote fetches all feedback notes for (ticketID, commitHash) from the DB,
// marshals them to a JSON array, and writes them as a git note on that commit.
// If no notes remain, the git note is removed. Errors are silently ignored because
// git note sync is best-effort — the DB is the source of truth.
func syncGitNote(database *db.DB, ticketID int64, commitHash string) {
	if commitHash == "" {
		return
	}
	ticket, err := database.GetTicketByID(ticketID)
	if err != nil || ticket.WorktreePath == "" {
		return
	}
	// Fetch all notes for this commit across all hunks/files.
	notes, err := database.ListFeedbackNotesByTicket(ticketID)
	if err != nil {
		return
	}
	// Filter to only notes for this specific commit.
	var commitNotes []db.FeedbackNote
	for _, n := range notes {
		if n.CommitHash == commitHash {
			commitNotes = append(commitNotes, n)
		}
	}
	if len(commitNotes) == 0 {
		git.NoteRemove(ticket.WorktreePath, commitHash)
		return
	}
	b, err := json.Marshal(commitNotes)
	if err != nil {
		return
	}
	git.NoteSet(ticket.WorktreePath, commitHash, string(b))
}
