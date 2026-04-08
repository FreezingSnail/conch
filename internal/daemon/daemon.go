// Package daemon is a Unix-socket JSON server. It receives Request messages,
// dispatches them to the db, git, and config packages, and returns Response messages.
package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/config"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
	"github.com/FreezingSnail/conch/internal/harness"
)

// SockAddr returns the canonical Unix socket path under $HOME/.conch.
func SockAddr() string {
	return filepath.Join(os.Getenv("HOME"), ".conch", "daemon.sock")
}

// Run listens on the Unix socket and blocks forever, spawning a goroutine per connection.
func Run(database *db.DB) error {
	addr := SockAddr()
	os.Remove(addr)
	ln, err := net.Listen("unix", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()
	fmt.Println("conchd: listening on", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go handle(conn, database)
	}
}

func handle(conn net.Conn, database *db.DB) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req client.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeResp(conn, client.Response{Error: "invalid json"})
			continue
		}
		writeResp(conn, dispatch(req, database))
	}
}

// dispatch is the central routing function. Unknown actions return an error
// response rather than panicking.
func dispatch(req client.Request, database *db.DB) client.Response {
	switch req.Action {
	case "ping":
		return client.Response{OK: true}

	case "create_session":
		h := req.Harness
		if h == "" {
			h = "kiro"
		}
		s := req.Status
		if s == "" {
			s = "completed"
		}
		id, err := database.CreateSession(h, s)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, ID: id}

	case "execute":
		if req.Prompt == "" {
			return client.Response{Error: "prompt required"}
		}
		id, err := database.CreateSession("kiro", "running")
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		go runBackground(id, req.Prompt, database)
		return client.Response{OK: true, ID: id}

	case "list_sessions":
		sessions, err := database.ListSessions()
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, Sessions: sessions}

	case "list_tickets":
		tickets, err := database.ListTickets()
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, Tickets: tickets}

	case "create_task":
		if req.TicketID == 0 || req.Title == "" {
			return client.Response{Error: "ticket_id and title required"}
		}
		id, err := database.CreateTaskWithBody(req.TicketID, req.Title, req.Body)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, ID: id}

	case "get_task":
		if req.TaskID == 0 {
			return client.Response{Error: "task_id required"}
		}
		t, err := database.GetTask(req.TaskID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, Task: &t}

	case "update_task_status":
		if req.TaskID == 0 || req.Status == "" {
			return client.Response{Error: "task_id and status required"}
		}
		if err := database.UpdateTaskStatus(req.TaskID, req.Status); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "list_tasks":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}
		}
		tasks, err := database.ListTasksByTicket(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, Tasks: tasks}

	case "add_dep":
		if req.BlockerID == 0 || req.BlockedID == 0 {
			return client.Response{Error: "blocker_id and blocked_id required"}
		}
		if err := database.AddDependency(req.BlockerID, req.BlockedID); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "remove_dep":
		if req.BlockerID == 0 || req.BlockedID == 0 {
			return client.Response{Error: "blocker_id and blocked_id required"}
		}
		if err := database.RemoveDependency(req.BlockerID, req.BlockedID); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "list_blocked_by":
		if req.TaskID == 0 {
			return client.Response{Error: "task_id required"}
		}
		tasks, err := database.ListBlockedBy(req.TaskID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, Tasks: tasks}

	case "list_blocks":
		if req.TaskID == 0 {
			return client.Response{Error: "task_id required"}
		}
		tasks, err := database.ListBlocks(req.TaskID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, Tasks: tasks}

	case "create_ticket":
		if req.Title == "" {
			return client.Response{Error: "title required"}
		}
		id, err := database.CreateTicket(req.Title, req.Description, req.Repo)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		if req.Repo != "" {
			wtPath := filepath.Join(os.Getenv("HOME"), ".conch", "worktrees", fmt.Sprintf("%d", id))
			if err := git.WorktreeAdd(req.Repo, wtPath, fmt.Sprintf("%d", id)); err != nil {
				return client.Response{Error: "ticket created but worktree failed: " + err.Error()}
			}
			database.SetTicketRepo(id, req.Repo, wtPath)
		}
		return client.Response{OK: true, ID: id}

	case "list_repos":
		cfg, err := config.Load()
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		repos, err := config.FindRepos(cfg.WorkDirs)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, Repos: repos}

	case "create_worktree":
		if req.TicketID == 0 || req.Repo == "" {
			return client.Response{Error: "ticket_id and repo required"}
		}
		wtPath := filepath.Join(os.Getenv("HOME"), ".conch", "worktrees", fmt.Sprintf("%d", req.TicketID))
		if err := git.WorktreeAdd(req.Repo, wtPath, fmt.Sprintf("%d", req.TicketID)); err != nil {
			return client.Response{Error: err.Error()}
		}
		if err := database.SetTicketRepo(req.TicketID, req.Repo, wtPath); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "remove_worktree":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}
		}
		ticket, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		if ticket.WorktreePath == "" {
			return client.Response{Error: "no worktree for ticket"}
		}
		if err := git.WorktreeRemove(ticket.Repo, ticket.WorktreePath); err != nil {
			return client.Response{Error: err.Error()}
		}
		database.SetTicketRepo(req.TicketID, ticket.Repo, "")
		return client.Response{OK: true}

	case "list_worktrees":
		tickets, err := database.ListTickets()
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		var active []db.Ticket
		for _, t := range tickets {
			if t.WorktreePath != "" {
				active = append(active, t)
			}
		}
		return client.Response{OK: true, Tickets: active}

	case "sync_worktrees":
		tickets, err := database.ListTickets()
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		for _, t := range tickets {
			if t.WorktreePath == "" {
				continue
			}
			git.FetchMain(t.Repo)
			git.RebaseOntoMain(t.Repo, t.WorktreePath)
		}
		return client.Response{OK: true}

	case "push_worktree":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		branch := fmt.Sprintf("%d", t.ID)
		if err := git.Push(t.WorktreePath, branch); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "open_pr":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		branch := fmt.Sprintf("%d", t.ID)
		base := git.DefaultBranch(t.Repo)
		cmd := exec.Command("gh", "pr", "create", "--head", branch, "--base", base, "--fill")
		cmd.Dir = t.WorktreePath
		out, err := cmd.CombinedOutput()
		if err != nil {
			return client.Response{Error: string(out)}
		}
		return client.Response{OK: true, Lines: []string{string(out)}}

	case "worktree_status":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		cmd := exec.Command("git", "status")
		cmd.Dir = t.WorktreePath
		out, _ := cmd.CombinedOutput()
		return client.Response{OK: true, Lines: []string{string(out)}}

	case "worktree_diff":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		cmd := exec.Command("git", "diff")
		cmd.Dir = t.WorktreePath
		out, _ := cmd.CombinedOutput()
		return client.Response{OK: true, Lines: []string{string(out)}}

	default:
		return client.Response{Error: "unknown action"}
	}
}

// runBackground streams stdout line-by-line into session_logs and updates the
// session status to "completed" or "error" when the process exits.
func runBackground(sessionID int64, prompt string, database *db.DB) {
	h := harness.Get("kiro")
	cmd := h.Background(prompt)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		return
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		database.AppendSessionLog(sessionID, "error", err.Error())
		return
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		database.AppendSessionLog(sessionID, "stdout", scanner.Text())
	}
	if err := cmd.Wait(); err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		database.AppendSessionLog(sessionID, "exit_error", err.Error())
		return
	}
	database.UpdateSessionStatus(sessionID, "completed")
	_ = io.Discard
}

func writeResp(w io.Writer, r client.Response) {
	b, _ := json.Marshal(r)
	b = append(b, '\n')
	w.Write(b)
}
