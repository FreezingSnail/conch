package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
)

// handleTasks routes task and dependency actions. Returns (resp, true) for known
// actions, (zero, false) otherwise.
func handleTasks(req client.Request, database *db.DB) (client.Response, bool) {
	switch req.Action {
	case "create_task":
		if req.TicketID == 0 || req.Title == "" {
			return client.Response{Error: "ticket_id and title required"}, true
		}
		id, err := database.CreateTaskWithBody(req.TicketID, req.Title, req.Body)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, ID: id}, true

	case "get_task":
		if req.TaskID == 0 {
			return client.Response{Error: "task_id required"}, true
		}
		t, err := database.GetTask(req.TaskID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, Task: &t}, true

	case "update_task_status":
		if req.TaskID == 0 || req.Status == "" {
			return client.Response{Error: "task_id and status required"}, true
		}
		if err := database.UpdateTaskStatus(req.TaskID, req.Status); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "list_tasks":
		if req.TicketID == 0 {
			return client.Response{Error: "ticket_id required"}, true
		}
		tasks, err := database.ListTasksByTicket(req.TicketID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, Tasks: tasks}, true

	case "add_dep":
		if req.BlockerID == 0 || req.BlockedID == 0 {
			return client.Response{Error: "blocker_id and blocked_id required"}, true
		}
		if err := database.AddDependency(req.BlockerID, req.BlockedID); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "remove_dep":
		if req.BlockerID == 0 || req.BlockedID == 0 {
			return client.Response{Error: "blocker_id and blocked_id required"}, true
		}
		if err := database.RemoveDependency(req.BlockerID, req.BlockedID); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "list_blocked_by":
		if req.TaskID == 0 {
			return client.Response{Error: "task_id required"}, true
		}
		tasks, err := database.ListBlockedBy(req.TaskID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, Tasks: tasks}, true

	case "list_blocks":
		if req.TaskID == 0 {
			return client.Response{Error: "task_id required"}, true
		}
		tasks, err := database.ListBlocks(req.TaskID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, Tasks: tasks}, true

	case "import_tasks":
		if req.TicketID == 0 || req.Dir == "" {
			return client.Response{Error: "ticket_id and dir required"}, true
		}
		tasks, err := importTasks(req.TicketID, req.Dir, database)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, Tasks: tasks}, true

	default:
		return client.Response{}, false
	}
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
