// Package daemon is a Unix-socket JSON server. It receives Request messages,
// dispatches them to the db, git, and config packages, and returns Response messages.
package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/FreezingSnail/conch/internal/assets"
	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/config"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
	"github.com/FreezingSnail/conch/internal/kiro"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// seedKiroConfig writes embedded kiro config files into the worktree.
// Non-fatal: worktree is still usable without them.
func seedKiroConfig(worktreePath string) {
	settingsDir := filepath.Join(worktreePath, ".kiro", "settings")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return
	}
	os.WriteFile(filepath.Join(settingsDir, "lsp.json"), assets.KiroLSPConfig, 0o644) //nolint:errcheck

	agentsDir := filepath.Join(worktreePath, ".kiro", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return
	}
	os.WriteFile(filepath.Join(agentsDir, "executor.json"), assets.KiroExecutorAgent, 0o644)       //nolint:errcheck
	os.WriteFile(filepath.Join(agentsDir, "implementor.json"), assets.KiroImplementorAgent, 0o644) //nolint:errcheck
}

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

// ticketBranch returns the git branch name for a ticket. plan_setup uses
// TicketNumber as the branch; older paths use the numeric ID.
func ticketBranch(t db.Ticket) string {
	if t.TicketNumber != "" {
		return t.TicketNumber
	}
	return fmt.Sprintf("%d", t.ID)
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
		id, err := database.CreateSession(0, h, s)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, ID: id}

	case "execute":
		if req.Prompt == "" {
			return client.Response{Error: "prompt required"}
		}
		id, err := database.CreateSession(0, "kiro", "running")
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

	case "list_session_logs":
		if req.SessionID == 0 {
			return client.Response{Error: "session_id required"}
		}
		logs, err := database.ListSessionLogs(req.SessionID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, SessionLogs: logs}

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
		id, err := database.CreateTicket(req.TicketNumber, req.Title, req.Description, req.Repo)
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
		seedKiroConfig(wtPath)
		return client.Response{OK: true}

	case "delete_ticket":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		if t.WorktreePath != "" {
			git.WorktreeRemove(t.Repo, t.WorktreePath)
		}
		if err := database.DeleteTicket(req.TicketID); err != nil {
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
		database.DeleteWorktreeByPath(ticket.WorktreePath)
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
		branch := ticketBranch(t)
		if err := git.Push(t.WorktreePath, branch); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "merge_worktree":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		branch := ticketBranch(t)
		out, mergeErr := git.MergeIntoMain(t.Repo, branch)
		if mergeErr != nil {
			// Leave repo in conflicted state; surface output to the user.
			return client.Response{OK: true, Lines: []string{out}}
		}
		git.WorktreeRemove(t.Repo, t.WorktreePath)
		database.SetTicketRepo(req.TicketID, t.Repo, "")
		database.DeleteWorktreeByPath(t.WorktreePath)
		return client.Response{OK: true}

	case "open_pr":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}
		}
		t, err := database.GetTicketByID(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		branch := ticketBranch(t)
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

	case "plan_setup":
		// plan_setup atomically creates a ticket, one worktree per repo, and a
		// running session. Returns ticket ID and session ID.
		if req.TicketNumber == "" || req.Title == "" || len(req.Repos) == 0 {
			return client.Response{Error: "ticket_number, title, and repos required"}
		}
		ticketID, err := database.CreateTicket(req.TicketNumber, req.Title, req.Description, req.Repos[0])
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		for _, repo := range req.Repos {
			base := filepath.Base(repo)
			wtPath := filepath.Join(os.Getenv("HOME"), ".conch", "worktrees", req.TicketNumber, base)
			if err := git.WorktreeAdd(repo, wtPath, req.TicketNumber); err != nil {
				return client.Response{Error: fmt.Sprintf("worktree for %s: %s", base, err.Error())}
			}
			if err := database.CreateWorktree(ticketID, repo, wtPath); err != nil {
				return client.Response{Error: err.Error()}
			}
			if err := database.SetTicketRepo(ticketID, repo, wtPath); err != nil {
				return client.Response{Error: err.Error()}
			}
			seedKiroConfig(wtPath)
		}
		sessionID, err := database.CreateSession(ticketID, "kiro", "running")
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, ID: ticketID, SessionID: sessionID}

	case "set_kiro_session":
		// set_kiro_session stores the kiro-cli UUID on an existing session row.
		if req.SessionID == 0 || req.KiroSessionID == "" {
			return client.Response{Error: "session_id and kiro_session_id required"}
		}
		if err := database.SetSessionKiroID(req.SessionID, req.KiroSessionID); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "kill_session":
		if req.SessionID == 0 {
			return client.Response{Error: "session_id required"}
		}
		if err := database.UpdateSessionStatus(req.SessionID, "error"); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "update_session_status":
		if req.SessionID == 0 || req.Status == "" {
			return client.Response{Error: "session_id and status required"}
		}
		if err := database.UpdateSessionStatus(req.SessionID, req.Status); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "plan_complete":
		// plan_complete is called by the tmux wrapper script after kiro exits.
		// It diffs kiro sessions to capture the new UUID, then marks the session completed.
		if req.SessionID == 0 || req.Worktree == "" {
			return client.Response{Error: "session_id and worktree required"}
		}
		var before []string
		if req.BeforeIDs != "" {
			before = strings.Split(req.BeforeIDs, ",")
		}
		afterIDs := kiro.ListSessionIDs(req.Worktree)
		if uuid := kiro.NewSessionID(before, afterIDs); uuid != "" {
			database.SetSessionKiroID(req.SessionID, uuid) //nolint:errcheck
		}
		if err := database.UpdateSessionStatus(req.SessionID, "completed"); err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true}

	case "exec_start":
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
		// Block duplicate launches.
		sessions, err := database.ListSessions()
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		for _, s := range sessions {
			if s.TicketID == req.TicketID && s.Status == "running" {
				return client.Response{Error: "executor already running for this ticket"}
			}
		}
		tasks, err := database.ListTasksByTicket(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		prompt := kiro.BuildExecutorPrompt(ticket.TicketNumber, ticket.ID, tasks)
		sessionID, err := database.CreateSession(req.TicketID, "kiro-executor", "running")
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		go runExecutor(sessionID, prompt, ticket.WorktreePath, database)
		return client.Response{OK: true, ID: sessionID}

	case "import_tasks":
		if req.TicketID == 0 || req.Dir == "" {
			return client.Response{Error: "ticket_id and dir required"}
		}
		tasks, err := importTasks(req.TicketID, req.Dir, database)
		if err != nil {
			return client.Response{Error: err.Error()}
		}
		return client.Response{OK: true, Tasks: tasks}

	default:
		return client.Response{Error: "unknown action"}
	}
}

// runBackground streams stdout line-by-line into session_logs and updates the
// session status to "completed" or "error" when the process exits.
func runBackground(sessionID int64, prompt string, database *db.DB) {
	cmd := kiro.Kiro{}.Background(prompt)
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

// importTasks walks dir for *.code-task.md files, creates tasks with an
// augmented ## Tests section, and wires step-based dependencies.
func importTasks(ticketID int64, dir string, database *db.DB) ([]db.Task, error) {
	// Collect files grouped by step directory, sorted by path.
	stepFiles := map[string][]string{}
	var stepOrder []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".code-task.md") {
			return err
		}
		step := filepath.Base(filepath.Dir(path))
		if _, seen := stepFiles[step]; !seen {
			stepOrder = append(stepOrder, step)
		}
		stepFiles[step] = append(stepFiles[step], path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(stepOrder)
	for _, step := range stepOrder {
		sort.Strings(stepFiles[step])
	}

	// Create tasks, grouped by step so we can wire deps.
	stepIDs := map[string][]int64{}
	var all []db.Task
	for _, step := range stepOrder {
		for _, path := range stepFiles[step] {
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			title, body := parseCodeTask(string(raw))
			augmented := body + "\n" + augmentTests(body)
			id, err := database.CreateTaskWithBody(ticketID, title, augmented)
			if err != nil {
				return nil, err
			}
			stepIDs[step] = append(stepIDs[step], id)
			all = append(all, db.Task{ID: id, TicketID: ticketID, Title: title, Body: augmented, Status: "todo"})
		}
	}

	// Wire cross-step dependencies.
	for i := 1; i < len(stepOrder); i++ {
		prev, cur := stepOrder[i-1], stepOrder[i]
		for _, blocker := range stepIDs[prev] {
			for _, blocked := range stepIDs[cur] {
				if err := database.AddDependency(blocker, blocked); err != nil {
					return nil, err
				}
			}
		}
	}
	return all, nil
}

// parseCodeTask extracts the title from the `# Task: <title>` first line and
// returns the remainder as the body.
func parseCodeTask(content string) (title, body string) {
	content = strings.TrimSpace(content)
	idx := strings.Index(content, "\n")
	if idx == -1 {
		return strings.TrimPrefix(content, "# Task: "), ""
	}
	title = strings.TrimPrefix(strings.TrimSpace(content[:idx]), "# Task: ")
	body = strings.TrimSpace(content[idx+1:])
	return
}

// augmentTests derives a ## Tests section from the task body. It scans for
// requirement lines and produces named test cases covering success, failure,
// and any explicit edge cases.
func augmentTests(body string) string {
	var cases []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "* ") {
			continue
		}
		text := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
		text = strings.TrimSpace(text)
		if text == "" || strings.HasPrefix(text, "`") {
			continue
		}
		words := strings.Fields(text)
		if len(words) > 5 {
			words = words[:5]
		}
		name := strings.ToLower(strings.Join(words, "_"))
		name = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
				return r
			}
			return '_'
		}, name)
		cases = append(cases, fmt.Sprintf("- `test_%s`: given %s, expect success", name, text))
	}
	cases = append(cases, "- `test_invalid_input`: given missing or invalid input, expect error")
	return "## Tests\n\n" + strings.Join(cases, "\n")
}

// runExecutor spawns a headless kiro executor in the ticket's worktree and
// updates the session status when the process exits.
func runExecutor(sessionID int64, prompt, worktreePath string, database *db.DB) {
	agentPath := filepath.Join(os.Getenv("HOME"), ".kiro", "agents", "executor.json")
	if _, err := os.Stat(agentPath); err != nil {
		database.AppendSessionLog(sessionID, "error", "executor agent not found: run tooling/link.sh")
		database.UpdateSessionStatus(sessionID, "error")
		return
	}
	cmd := kiro.Kiro{}.BackgroundWithAgent("executor", prompt, worktreePath)
	// Ensure conch binary is findable. The daemon may be launched without the
	// user's full PATH. Prepend the Go bin dir and the daemon's own dir.
	goBin := filepath.Join(os.Getenv("HOME"), "go", "bin")
	selfDir := filepath.Dir(os.Args[0])
	cmd.Env = append(os.Environ(), "PATH="+goBin+":"+selfDir+":"+os.Getenv("PATH"))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		return
	}
	cmd.Stderr = cmd.Stdout
	beforeIDs := kiro.ListSessionIDs(worktreePath)
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
	} else {
		database.UpdateSessionStatus(sessionID, "completed")
	}
	// Diff after exit — deterministic, no goroutine race.
	if uuid := kiro.NewSessionID(beforeIDs, kiro.ListSessionIDs(worktreePath)); uuid != "" {
		database.SetSessionKiroID(sessionID, uuid) //nolint:errcheck
	}
}
