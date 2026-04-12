package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/config"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
)

// handleWorktrees routes worktree actions. Returns (resp, true) for known
// actions, (zero, false) otherwise.
func handleWorktrees(req client.Request, database *db.DB) (client.Response, bool) {
	switch req.Action {
	case "create_worktree":
		if req.TicketID == 0 || req.Repo == "" {
			return client.Response{Error: "ticket_id and repo required"}, true
		}
		wtPath := filepath.Join(os.Getenv("HOME"), ".conch", "worktrees", filepath.Base(req.Repo), fmt.Sprintf("%d", req.TicketID))
		if err := git.WorktreeAdd(req.Repo, wtPath, fmt.Sprintf("%d", req.TicketID)); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		if err := database.SetTicketRepo(req.TicketID, req.Repo, wtPath); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		slugCfg, _ := config.Load()
		seedKiroConfig(wtPath, slugCfg.EffectiveSlugMode())
		return client.Response{OK: true}, true

	case "remove_worktree":
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
		if err := git.WorktreeRemove(ticket.Repo, ticket.WorktreePath); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		database.SetTicketRepo(req.TicketID, ticket.Repo, "")
		database.DeleteWorktreeByPath(ticket.WorktreePath)
		return client.Response{OK: true}, true

	case "list_worktrees":
		tickets, err := database.ListTickets()
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		var active []db.Ticket
		for _, t := range tickets {
			if t.WorktreePath != "" {
				active = append(active, t)
			}
		}
		return client.Response{OK: true, Tickets: active}, true

	case "sync_worktrees":
		tickets, err := database.ListTickets()
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		for _, t := range tickets {
			if t.WorktreePath == "" {
				continue
			}
			git.FetchMain(t.Repo)
			git.RebaseOntoMain(t.Repo, t.WorktreePath)
		}
		return client.Response{OK: true}, true

	case "push_worktree":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		branch := ticketBranch(t)
		if err := git.Push(t.WorktreePath, branch); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "merge_worktree":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		branch := ticketBranch(t)
		out, mergeErr := git.MergeIntoMain(t.Repo, branch)
		if mergeErr != nil {
			return client.Response{OK: true, Lines: []string{out}}, true
		}
		git.WorktreeRemove(t.Repo, t.WorktreePath)
		database.SetTicketRepo(req.TicketID, t.Repo, "")
		database.DeleteWorktreeByPath(t.WorktreePath)
		return client.Response{OK: true}, true

	case "open_pr":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		branch := ticketBranch(t)
		base := git.DefaultBranch(t.Repo)
		cmd := exec.Command("gh", "pr", "create", "--head", branch, "--base", base, "--fill")
		cmd.Dir = t.WorktreePath
		out, err := cmd.CombinedOutput()
		if err != nil {
			return client.Response{Error: string(out)}, true
		}
		return client.Response{OK: true, Lines: []string{string(out)}}, true

	case "worktree_status":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		cmd := exec.Command("git", "status")
		cmd.Dir = t.WorktreePath
		out, _ := cmd.CombinedOutput()
		return client.Response{OK: true, Lines: []string{string(out)}}, true

	case "worktree_diff":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		cmd := exec.Command("git", "diff")
		cmd.Dir = t.WorktreePath
		out, _ := cmd.CombinedOutput()
		return client.Response{OK: true, Lines: []string{string(out)}}, true

	default:
		return client.Response{}, false
	}
}
