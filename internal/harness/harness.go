// Package harness defines the Harness interface and shared tmux helpers.
package harness

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
}

// InTmux reports whether the current process is running inside a tmux session.
func InTmux() bool { return os.Getenv("TMUX") != "" }

// JoinIDs joins a slice of session IDs into the comma-separated string used by
// the notify callback.
func JoinIDs(ids []string) string { return strings.Join(ids, ",") }

// SpawnTmuxPane opens a new tmux pane running h, then calls back to the conch
// daemon via `conch notify` when the harness exits.
// agent and prompt may be empty to launch the interactive session picker.
func SpawnTmuxPane(h Harness, agent, prompt, dir string, sessionID int64, beforeIDs string) error {
	notify := fmt.Sprintf("conch notify --session-id %d --worktree %q --before-ids %q",
		sessionID, dir, beforeIDs)
	shellCmd := fmt.Sprintf("cd %q && %s; %s", dir, h.CLICommand(agent, prompt), notify)
	return exec.Command("tmux", "split-window", "-h", shellCmd).Run()
}

// SpawnTmuxWindow opens a new named tmux window running h. On exit it calls
// conch notify, switches focus back to the originating window, then closes itself.
func SpawnTmuxWindow(h Harness, windowName, agent, prompt, dir string, sessionID int64, beforeIDs string) error {
	prevID, err := exec.Command("tmux", "display-message", "-p", "#{window_id}").Output()
	if err != nil {
		return fmt.Errorf("tmux display-message: %w", err)
	}
	prev := strings.TrimSpace(string(prevID))
	notify := fmt.Sprintf("conch notify --session-id %d --worktree %q --before-ids %q", sessionID, dir, beforeIDs)
	shellCmd := fmt.Sprintf("SELF=$(tmux display-message -p '#{window_id}'); cd %q && %s; %s; tmux select-window -t %s; tmux kill-window -t $SELF",
		dir, h.CLICommand(agent, prompt), notify, prev)
	return exec.Command("tmux", "new-window", "-n", windowName, "sh", "-c", shellCmd).Run()
}

// SpawnTmuxPaneResume opens a new tmux pane with the harness session picker in dir.
func SpawnTmuxPaneResume(h Harness, dir string) error {
	shellCmd := fmt.Sprintf("cd %q && %s", dir, h.CLICommand("", ""))
	return exec.Command("tmux", "split-window", "-h", shellCmd).Run()
}
