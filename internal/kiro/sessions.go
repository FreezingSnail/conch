// Package kiro wraps kiro-cli invocations used by conch.
package kiro

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/FreezingSnail/conch/internal/db"
)

var uuidRe = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// ListSessionIDs runs "kiro-cli chat --list-sessions" in dir and returns all
// session UUIDs found in the output. Returns nil on error so callers can always
// diff safely.
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
// Returns "" if no new session is found.
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

// BuildPrompt formats the planning context string passed to kiro-cli as the
// initial query.
func BuildPrompt(ticketNumber string, ticketID int64, desc, context string) string {
	s := fmt.Sprintf("[CONCH PLANNING] ticket:%s id:%d\ndesc:%s", ticketNumber, ticketID, desc)
	if strings.TrimSpace(context) != "" {
		s += "\ncontext:" + context
	}
	return s
}

// BuildExecutorPrompt builds the initial prompt for a headless executor session.
// It inlines the task list so the agent doesn't need a tool call to start.
func BuildExecutorPrompt(ticketNumber string, ticketID int64, tasks []db.Task) string {
	lines := make([]string, len(tasks))
	for i, t := range tasks {
		lines[i] = fmt.Sprintf("  [%s] id:%d %s", t.Status, t.ID, t.Title)
	}
	return fmt.Sprintf("[CONCH EXECUTOR] ticket:%s id:%d\ntasks:\n%s",
		ticketNumber, ticketID, strings.Join(lines, "\n"))
}
