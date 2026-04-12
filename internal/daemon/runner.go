package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/kiro"
)

// commitSignal is the JSON artifact written by an implementor agent to signal
// successful task completion. It carries the conventional-commit metadata
// needed to produce a git commit.
type commitSignal struct {
	TaskID  int64  `json:"task_id"`
	Type    string `json:"type"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// taskResult carries the outcome of a single runImplementor goroutine.
type taskResult struct {
	taskID  int64
	success bool
	signal  commitSignal
	errMsg  string
}

// typePrecedence maps conventional-commit types to a numeric priority.
// Lower value = higher precedence.
var typePrecedence = map[string]int{
	"feat":     0,
	"fix":      1,
	"refactor": 2,
	"chore":    3,
	"test":     4,
	"docs":     5,
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
	stderr, err := cmd.StderrPipe()
	if err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		return
	}
	combined := io.MultiReader(stdout, stderr)
	if err := cmd.Start(); err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		database.AppendSessionLog(sessionID, "error", err.Error())
		return
	}
	scanner := bufio.NewScanner(combined)
	for scanner.Scan() {
		database.AppendSessionLog(sessionID, "stdout", scanner.Text())
	}
	if err := cmd.Wait(); err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		database.AppendSessionLog(sessionID, "exit_error", err.Error())
		return
	}
	database.UpdateSessionStatus(sessionID, "completed")
}

// runGoExecutor drives the full task execution loop in Go. It repeatedly finds
// executable tasks (todo/in-progress with all blockers done), spawns one
// implementor goroutine per task, collects results, commits successful batches,
// and marks tasks done or human-intervention accordingly. The session is
// finalized to "completed" or "error" when the loop exits.
func runGoExecutor(sessionID, ticketID int64, worktreePath string, database *db.DB) {
	for {
		tasks, err := database.ListTasksByTicket(ticketID)
		if err != nil {
			database.UpdateSessionStatus(sessionID, "error")
			return
		}

		// Collect executable tasks: todo/in-progress with all blockers done.
		var executable []db.Task
		for _, t := range tasks {
			if t.Status != "todo" && t.Status != "in-progress" {
				continue
			}
			blockers, err := database.ListBlockedBy(t.ID)
			if err != nil {
				database.UpdateSessionStatus(sessionID, "error")
				return
			}
			allDone := true
			for _, b := range blockers {
				if b.Status != "done" {
					allDone = false
					break
				}
			}
			if allDone {
				executable = append(executable, t)
			}
		}

		if len(executable) == 0 {
			break
		}

		// Mark all executable tasks in-progress before spawning goroutines.
		for _, t := range executable {
			database.UpdateTaskStatus(t.ID, "in-progress")
		}

		results := make(chan taskResult, len(executable))
		var wg sync.WaitGroup
		for _, t := range executable {
			wg.Add(1)
			go runImplementor(sessionID, t, worktreePath, database, results, &wg)
		}
		wg.Wait()
		close(results)

		var successes []taskResult
		for r := range results {
			if r.success {
				successes = append(successes, r)
			} else {
				database.UpdateTaskStatus(r.taskID, "human-intervention")
				database.AppendSessionLog(sessionID, "stdout", fmt.Sprintf("task %d failed: %s", r.taskID, r.errMsg))
			}
		}

		if len(successes) > 0 {
			sig := synthesizeCommit(successes)
			if err := runGitCommit(worktreePath, sig); err != nil {
				// Commit failed — all successful tasks need human intervention.
				for _, r := range successes {
					database.UpdateTaskStatus(r.taskID, "human-intervention")
				}
				database.AppendSessionLog(sessionID, "stdout", fmt.Sprintf("git commit failed: %s", err.Error()))
			} else {
				for _, r := range successes {
					database.UpdateTaskStatus(r.taskID, "done")
				}
			}
		}
	}

	database.UpdateSessionStatus(sessionID, "completed")
}

// runImplementor spawns a kiro implementor agent for a single task, streams
// its output to session logs, then reads the JSON artifact the agent writes to
// .conch/task-<id>.complete.json. Sends a taskResult to results and calls
// wg.Done when finished.
func runImplementor(sessionID int64, task db.Task, worktreePath string, database *db.DB, results chan<- taskResult, wg *sync.WaitGroup) {
	defer wg.Done()

	prompt := kiro.BuildImplementorPrompt(task)
	cmd := kiro.Kiro{}.BackgroundWithAgent("implementor", prompt, worktreePath)

	// Ensure conch binary is findable without the user's full PATH.
	goBin := filepath.Join(os.Getenv("HOME"), "go", "bin")
	selfDir := filepath.Dir(os.Args[0])
	cmd.Env = append(os.Environ(), "PATH="+goBin+":"+selfDir+":"+os.Getenv("PATH"))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		results <- taskResult{taskID: task.ID, success: false, errMsg: err.Error()}
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		results <- taskResult{taskID: task.ID, success: false, errMsg: err.Error()}
		return
	}
	combined := io.MultiReader(stdout, stderr)

	if err := cmd.Start(); err != nil {
		results <- taskResult{taskID: task.ID, success: false, errMsg: err.Error()}
		return
	}

	scanner := bufio.NewScanner(combined)
	for scanner.Scan() {
		database.AppendSessionLog(sessionID, "stdout", scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		results <- taskResult{taskID: task.ID, success: false, errMsg: err.Error()}
		return
	}

	// Read the completion artifact written by the implementor agent.
	artifactPath := filepath.Join(worktreePath, ".conch", fmt.Sprintf("task-%d.complete.json", task.ID))
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		results <- taskResult{taskID: task.ID, success: false, errMsg: fmt.Sprintf("artifact missing: %s", err.Error())}
		return
	}

	var sig commitSignal
	if err := json.Unmarshal(data, &sig); err != nil {
		results <- taskResult{taskID: task.ID, success: false, errMsg: fmt.Sprintf("bad artifact JSON: %s", err.Error())}
		return
	}

	results <- taskResult{taskID: task.ID, success: true, signal: sig}
}

// synthesizeCommit merges one or more task results into a single commitSignal.
// For a single result the signal is returned as-is with "Tasks: <id>" appended
// to the body. For multiple results the highest-precedence type wins, the first
// result's subject is used (with " and N more tasks" when N>1), bodies are
// joined with newlines, and all IDs are listed in a "Tasks:" trailer.
func synthesizeCommit(results []taskResult) commitSignal {
	if len(results) == 1 {
		r := results[0]
		body := r.signal.Body
		if body != "" {
			body += "\n"
		}
		body += fmt.Sprintf("Tasks: %d", r.taskID)
		return commitSignal{
			TaskID:  r.taskID,
			Type:    r.signal.Type,
			Subject: r.signal.Subject,
			Body:    body,
		}
	}

	// Pick highest-precedence type.
	bestType := results[0].signal.Type
	bestPri, ok := typePrecedence[bestType]
	if !ok {
		bestPri = 999
	}
	for _, r := range results[1:] {
		pri, ok := typePrecedence[r.signal.Type]
		if !ok {
			pri = 999
		}
		if pri < bestPri {
			bestPri = pri
			bestType = r.signal.Type
		}
	}

	// Subject: first result's subject + " and N more tasks" if N>1.
	subject := results[0].signal.Subject
	if len(results) > 1 {
		subject = fmt.Sprintf("%s and %d more tasks", subject, len(results)-1)
	}

	// Body: join all bodies, then append Tasks trailer.
	var bodies []string
	var ids []string
	for _, r := range results {
		if r.signal.Body != "" {
			bodies = append(bodies, r.signal.Body)
		}
		ids = append(ids, fmt.Sprintf("%d", r.taskID))
	}
	body := strings.Join(bodies, "\n")
	if body != "" {
		body += "\n"
	}
	body += "Tasks: " + strings.Join(ids, ", ")

	return commitSignal{
		Type:    bestType,
		Subject: subject,
		Body:    body,
	}
}

// runGitCommit stages all changes and creates a conventional commit in the
// given worktree. Returns a non-nil error (with combined output) on failure.
func runGitCommit(worktreePath string, sig commitSignal) error {
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = worktreePath
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s", strings.TrimSpace(string(out)))
	}

	message := fmt.Sprintf("%s: %s", sig.Type, sig.Subject)
	commitCmd := exec.Command("git", "commit", "-m", message, "-m", sig.Body)
	commitCmd.Dir = worktreePath
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
