package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/config"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
)

// handleTickets routes ticket and repo actions. Returns (resp, true) for known
// actions, (zero, false) otherwise.
func handleTickets(req client.Request, database *db.DB) (client.Response, bool) {
	switch req.Action {
	case "create_ticket":
		if req.Title == "" {
			return client.Response{Error: "title required"}, true
		}
		id, err := database.CreateTicket(req.TicketNumber, req.Title, req.Description, req.Repo)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		if req.Repo != "" {
			wtPath := filepath.Join(os.Getenv("HOME"), ".conch", "worktrees", filepath.Base(req.Repo), fmt.Sprintf("%d", id))
			if err := git.WorktreeAdd(req.Repo, wtPath, fmt.Sprintf("%d", id)); err != nil {
				return client.Response{Error: "ticket created but worktree failed: " + err.Error()}, true
			}
			database.SetTicketRepo(id, req.Repo, wtPath)
		}
		return client.Response{OK: true, ID: id}, true

	case "list_tickets":
		tickets, err := database.ListTickets()
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, Tickets: tickets}, true

	case "delete_ticket":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		if t.WorktreePath != "" {
			git.WorktreeRemove(t.Repo, t.WorktreePath)
		}
		if err := database.DeleteTicket(req.TicketID); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "list_repos":
		cfg, err := config.Load()
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		repos, err := config.FindRepos(cfg.WorkDirs)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, Repos: repos}, true

	default:
		return client.Response{}, false
	}
}
