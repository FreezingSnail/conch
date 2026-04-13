// Package harness defines the Harness interface and shared tmux helpers.
package harness

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/FreezingSnail/conch/internal/db"
)

// Harness abstracts a CLI agent that can run interactively or in the background.
type Harness interface {
	Name() string
	// Interactive returns a command that takes over the terminal.
	Interactive() *exec.Cmd
	// InteractiveWithAgent returns a command that takes over the terminal using
	// the named agent with an initial prompt, running in the given directory.
	InteractiveWithAgent(agent, prompt, dir string) *exec.Cmd
	// Background returns a command that runs headlessly with the given prompt.
	Background(prompt string) *exec.Cmd
	// BackgroundWithAgent returns a headless command using the named agent, running in dir.
	BackgroundWithAgent(agent, prompt, dir string) *exec.Cmd
	// CLICommand returns the shell command string for the given agent and prompt,
	// used by the shared tmux helpers to build the inner shell invocation.
	CLICommand(agent, prompt string) string

	// SeedWorktree writes any harness-specific config/agent files into the worktree.
	// Non-fatal: worktree is still usable without them.
	SeedWorktree(worktreePath, slugMode string)
	// FindSession returns the harness-native session ID created in cwd after the
	// given time, or empty string if not found / not applicable.
	FindSession(cwd string, after time.Time) (string, error)

	// Prompt builders — each harness formats prompts for its own CLI.
	PlanningPrompt(ticketNum string, id int64, title, idea string) string
	ReplanPrompt(ticketNum string, id int64, title, desc string, notes []db.FeedbackNote) string
	ImplementorPrompt(task db.Task) string
	PRReviewPrompt(prNum int, repo, diff string) string
}

// InTmux reports whether the current process is running inside a tmux session.
func InTmux() bool { return os.Getenv("TMUX") != "" }

// SpawnTmuxPane opens a new tmux pane running h, then calls back to the conch
// daemon via `conch notify` when the harness exits.
// agent and prompt may be empty to launch the interactive session picker.
func SpawnTmuxPane(h Harness, agent, prompt, dir string, sessionID int64) error {
	notify := fmt.Sprintf("conch notify --session-id %d --worktree %q", sessionID, dir)
	shellCmd := fmt.Sprintf("cd %q && %s; %s", dir, h.CLICommand(agent, prompt), notify)
	return exec.Command("tmux", "split-window", "-h", shellCmd).Run()
}

// SpawnTmuxWindow opens a new named tmux window running h. On exit it calls
// conch notify, switches focus back to the originating window, then closes itself.
func SpawnTmuxWindow(h Harness, windowName, agent, prompt, dir string, sessionID int64) error {
	prevID, err := exec.Command("tmux", "display-message", "-p", "#{window_id}").Output()
	if err != nil {
		return fmt.Errorf("tmux display-message: %w", err)
	}
	prev := strings.TrimSpace(string(prevID))
	notify := fmt.Sprintf("conch notify --session-id %d --worktree %q", sessionID, dir)
	shellCmd := fmt.Sprintf("SELF=$(tmux display-message -p '#{window_id}'); cd %q && %s; %s; tmux select-window -t %s; tmux kill-window -t $SELF",
		dir, h.CLICommand(agent, prompt), notify, prev)
	return exec.Command("tmux", "new-window", "-n", windowName, "sh", "-c", shellCmd).Run()
}

// SpawnTmuxPaneResume opens a new tmux pane with the harness session picker in dir.
func SpawnTmuxPaneResume(h Harness, dir string) error {
	shellCmd := fmt.Sprintf("cd %q && %s", dir, h.CLICommand("", ""))
	return exec.Command("tmux", "split-window", "-h", shellCmd).Run()
}
