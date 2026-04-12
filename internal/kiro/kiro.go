// Package kiro implements the harness.Harness interface for kiro-cli.
package kiro

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/harness"
)

// compile-time interface check
var _ harness.Harness = Kiro{}

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

// BuildPrompt formats the planning context string passed to kiro-cli.
// title and idea are optional; non-empty values are appended as labelled lines.
func BuildPrompt(ticketNumber string, ticketID int64, title, idea string) string {
	s := fmt.Sprintf("[CONCH PLANNING] ticket:%s id:%d", ticketNumber, ticketID)
	if title != "" {
		s += "\ntitle: " + title
	}
	if idea != "" {
		s += "\nidea: " + idea
	}
	return s
}

// BuildReplanPrompt builds the prompt for a replanning session, including the
// ticket title/description and any unaddressed feedback notes as context.
func BuildReplanPrompt(ticketNumber string, ticketID int64, title, description string, notes []db.FeedbackNote) string {
	s := fmt.Sprintf("[CONCH PLANNING] ticket:%s id:%d", ticketNumber, ticketID)
	if title != "" {
		s += "\ntitle: " + title
	}
	if description != "" {
		s += "\nidea: " + description
	}
	if len(notes) == 0 {
		return s
	}
	lines := make([]string, len(notes))
	for i, n := range notes {
		lines[i] = fmt.Sprintf("- %s %s: %s", n.FilePath, n.HunkHeader, n.Body)
	}
	return s + "\n\nAdditional context (feedback notes from previous burrow session):\n" + strings.Join(lines, "\n")
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
