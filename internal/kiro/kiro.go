// Package kiro implements the harness.Harness interface for kiro-cli.
package kiro

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/harness"
)

// compile-time interface check
var _ harness.Harness = Kiro{}

var uuidRe = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// Kiro implements harness.Harness for kiro-cli.
type Kiro struct{}

func (k Kiro) Name() string { return "kiro" }

func (k Kiro) Interactive() *exec.Cmd {
	return exec.Command("kiro-cli", "chat")
}

func (k Kiro) InteractiveWithAgent(agent, prompt, dir string) *exec.Cmd {
	cmd := exec.Command("kiro-cli", "chat", "--agent", agent, "--trust-all-tools", prompt)
	cmd.Dir = dir
	return cmd
}

func (k Kiro) Background(prompt string) *exec.Cmd {
	return exec.Command("kiro-cli", "chat", "--no-interactive", "--trust-all-tools", prompt)
}

func (k Kiro) BackgroundWithAgent(agent, prompt, dir string) *exec.Cmd {
	cmd := exec.Command("kiro-cli", "chat", "--no-interactive", "--trust-all-tools", "--agent", agent, prompt)
	cmd.Dir = dir
	return cmd
}

// CLICommand returns the shell command string for use in tmux invocations.
// If agent is empty, returns the bare interactive command.
func (k Kiro) CLICommand(agent, prompt string) string {
	if agent == "" {
		return "kiro-cli chat"
	}
	return fmt.Sprintf("kiro-cli chat --agent %s --trust-all-tools %q", agent, prompt)
}

// ListSessionIDs runs "kiro-cli chat --list-sessions" in dir and returns all
// session UUIDs found in the output.
func ListSessionIDs(dir string) []string {
	cmd := exec.Command("kiro-cli", "chat", "--list-sessions")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	matches := uuidRe.FindAllString(string(out), -1)
	seen := make(map[string]bool, len(matches))
	result := make([]string, 0, len(matches))
	for _, id := range matches {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// NewSessionID returns the first UUID present in after but not in before.
func NewSessionID(before, after []string) string {
	old := make(map[string]bool, len(before))
	for _, id := range before {
		old[id] = true
	}
	for _, id := range after {
		if !old[id] {
			return id
		}
	}
	return ""
}

// BuildPrompt formats the planning context string passed to kiro-cli.
func BuildPrompt(ticketNumber string, ticketID int64) string {
	return fmt.Sprintf("[CONCH PLANNING] ticket:%s id:%d", ticketNumber, ticketID)
}

// BuildReplanPrompt builds the prompt for a replanning session, prepending any
// unaddressed feedback notes as additional context. If notes is empty the
// "Additional context" section is omitted so the prompt stays minimal.
func BuildReplanPrompt(ticketNumber string, ticketID int64, notes []db.FeedbackNote) string {
	header := fmt.Sprintf("[CONCH PLANNING] ticket:%s id:%d", ticketNumber, ticketID)
	if len(notes) == 0 {
		return header
	}
	lines := make([]string, len(notes))
	for i, n := range notes {
		lines[i] = fmt.Sprintf("- %s %s: %s", n.FilePath, n.HunkHeader, n.Body)
	}
	return header + "\n\nAdditional context (feedback notes from previous burrow session):\n" + strings.Join(lines, "\n")
}

// BuildPRReviewPrompt builds the prompt passed to the pr-reviewer agent.
func BuildPRReviewPrompt(prNumber int, repo, diff string) string {
	return fmt.Sprintf("[CONCH PR REVIEW] pr:%d repo:%s\n\n%s", prNumber, repo, diff)
}

// BuildExecutorPrompt builds the initial prompt for a headless executor session.
func BuildExecutorPrompt(ticketNumber string, ticketID int64, tasks []db.Task) string {
	lines := make([]string, len(tasks))
	for i, t := range tasks {
		lines[i] = fmt.Sprintf("  [%s] id:%d %s", t.Status, t.ID, t.Title)
	}
	return fmt.Sprintf("[CONCH EXECUTOR] ticket:%s id:%d\ntasks:\n%s",
		ticketNumber, ticketID, strings.Join(lines, "\n"))
}
