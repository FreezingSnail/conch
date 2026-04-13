package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/config"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
	"github.com/FreezingSnail/conch/internal/harness"
)

// handleSessions routes session lifecycle actions. Returns (resp, true) for
// known actions, (zero, false) otherwise.
func handleSessions(req client.Request, database *db.DB, h harness.Harness) (client.Response, bool) {
	switch req.Action {
	case "create_session":
		name := req.Harness
		if name == "" {
			name = h.Name()
		}
		s := req.Status
		if s == "" {
			s = "completed"
		}
		id, err := database.CreateSession(0, name, s)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, ID: id}, true

	case "execute":
		if req.Prompt == "" {
			return client.Response{Error: "prompt required"}, true
		}
		id, err := database.CreateSession(0, h.Name(), "running")
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		go runBackground(id, req.Prompt, database, h)
		return client.Response{OK: true, ID: id}, true

	case "list_sessions":
		sessions, err := database.ListSessions()
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, Sessions: sessions}, true

	case "list_session_logs":
		if req.SessionID == 0 {
			return client.Response{Error: "session_id required"}, true
		}
		logs, err := database.ListSessionLogs(req.SessionID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, SessionLogs: logs}, true

	case "kill_session":
		if req.SessionID == 0 {
			return client.Response{Error: "session_id required"}, true
		}
		if err := database.UpdateSessionStatus(req.SessionID, "error"); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "update_session_status":
		if req.SessionID == 0 || req.Status == "" {
			return client.Response{Error: "session_id and status required"}, true
		}
		if err := database.UpdateSessionStatus(req.SessionID, req.Status); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "set_kiro_session":
		if req.SessionID == 0 || req.KiroSessionID == "" {
			return client.Response{Error: "session_id and kiro_session_id required"}, true
		}
		if err := database.SetSessionKiroID(req.SessionID, req.KiroSessionID); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "plan_setup":
		if req.TicketNumber == "" || req.Title == "" || len(req.Repos) == 0 {
			return client.Response{Error: "ticket_number, title, and repos required"}, true
		}
		ticketID, err := database.CreateTicket(req.TicketNumber, req.Title, req.Description, req.Repos[0])
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		planCfg, _ := config.Load()
		for _, repo := range req.Repos {
			base := filepath.Base(repo)
			wtPath := filepath.Join(os.Getenv("HOME"), ".conch", "worktrees", base, req.TicketNumber)
			if err := git.WorktreeAdd(repo, wtPath, req.TicketNumber); err != nil {
				return client.Response{Error: fmt.Sprintf("worktree for %s: %s", base, err.Error())}, true
			}
			if err := database.CreateWorktree(ticketID, repo, wtPath); err != nil {
				return client.Response{Error: err.Error()}, true
			}
			if err := database.SetTicketRepo(ticketID, repo, wtPath); err != nil {
				return client.Response{Error: err.Error()}, true
			}
			h.SeedWorktree(wtPath, planCfg.EffectiveSlugMode())
		}
		sessionID, err := database.CreateSession(ticketID, h.Name(), "running")
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, ID: ticketID, SessionID: sessionID}, true

	case "plan_complete":
		if req.SessionID == 0 || req.Worktree == "" {
			return client.Response{Error: "session_id and worktree required"}, true
		}
		if sess, err := database.GetSessionByID(req.SessionID); err == nil {
			if uuid, err := h.FindSession(req.Worktree, sess.StartedAt); err == nil && uuid != "" {
				database.SetSessionKiroID(req.SessionID, uuid) //nolint:errcheck
			}
		}
		if err := database.UpdateSessionStatus(req.SessionID, "completed"); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "exec_start":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		ticket, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		if ticket.WorktreePath == "" {
			return client.Response{Error: "no worktree for ticket"}, true
		}
		sessions, err := database.ListSessions()
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		for _, s := range sessions {
			if s.TicketID == req.TicketID && s.Status == "running" {
				return client.Response{Error: "executor already running for this ticket"}, true
			}
		}
		sessionID, err := database.CreateSession(req.TicketID, h.Name()+"-executor", "running")
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		go runGoExecutor(sessionID, req.TicketID, ticket.WorktreePath, database, h)
		return client.Response{OK: true, ID: sessionID}, true

	case "replan_ticket":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		notes, err := database.ListFeedbackNotesByTicket(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		var unaddressed []db.FeedbackNote
		for _, n := range notes {
			if !n.Addressed {
				unaddressed = append(unaddressed, n)
			}
		}
		if err := database.MarkNotesAddressed(req.TicketID); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		ticket, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		if !harness.InTmux() {
			return client.Response{Error: "must be running inside tmux"}, true
		}
		prompt := h.ReplanPrompt(ticket.TicketNumber, ticket.ID, ticket.Title, ticket.Description, unaddressed)
		if err := harness.SpawnTmuxWindow(h, ticket.TicketNumber, "planning", prompt, ticket.WorktreePath, 0); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	default:
		return client.Response{}, false
	}
}
