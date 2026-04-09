// Package harness abstracts kiro-cli invocation for interactive and background sessions.
package harness

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// InTmux reports whether the current process is running inside a tmux session.
func InTmux() bool { return os.Getenv("TMUX") != "" }

// Harness abstracts a CLI agent that can run interactively or in the background.
type Harness interface {
	Name() string
	// Interactive returns a command that takes over the terminal.
	Interactive() *exec.Cmd
	// InteractiveWithAgent returns a command that takes over the terminal, using
	// the named agent with an initial prompt, running in the given directory.
	InteractiveWithAgent(agent, prompt, dir string) *exec.Cmd
	// Background returns a command that runs headlessly with the given prompt.
	Background(prompt string) *exec.Cmd
	// BackgroundWithAgent returns a headless command using the named agent, running in dir.
	BackgroundWithAgent(agent, prompt, dir string) *exec.Cmd
}

// Kiro implements Harness for kiro-cli.
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

// SpawnTmuxPane opens a new tmux pane running kiro-cli, then calls back to the
// conch daemon via `conch notify` when kiro exits.
// agent and prompt may be empty to launch the interactive session picker.
func (k Kiro) SpawnTmuxPane(agent, prompt, dir string, sessionID int64, beforeIDs string) error {
	var kiroCmd string
	if agent != "" {
		kiroCmd = fmt.Sprintf("kiro-cli chat --agent %s --trust-all-tools %q", agent, prompt)
	} else {
		kiroCmd = "kiro-cli chat"
	}
	notify := fmt.Sprintf("conch notify --session-id %d --worktree %q --before-ids %q",
		sessionID, dir, beforeIDs)
	// Run kiro; on exit (success or failure) call conch notify.
	shellCmd := fmt.Sprintf("cd %q && %s; %s", dir, kiroCmd, notify)
	return exec.Command("tmux", "split-window", "-h", shellCmd).Run()
}

// SpawnTmuxWindow opens a new named tmux window running kiro-cli. On exit it calls
// conch notify, switches focus back to the originating window, then closes itself.
func (k Kiro) SpawnTmuxWindow(windowName, agent, prompt, dir string, sessionID int64, beforeIDs string) error {
	prevID, err := exec.Command("tmux", "display-message", "-p", "#{window_id}").Output()
	if err != nil {
		return fmt.Errorf("tmux display-message: %w", err)
	}
	prev := strings.TrimSpace(string(prevID))
	var kiroCmd string
	if agent != "" {
		kiroCmd = fmt.Sprintf("kiro-cli chat --agent %s --trust-all-tools %q", agent, prompt)
	} else {
		kiroCmd = "kiro-cli chat"
	}
	notify := fmt.Sprintf("conch notify --session-id %d --worktree %q --before-ids %q", sessionID, dir, beforeIDs)
	shellCmd := fmt.Sprintf("SELF=$(tmux display-message -p '#{window_id}'); cd %q && %s; %s; tmux select-window -t %s; tmux kill-window -t $SELF", dir, kiroCmd, notify, prev)
	fmt.Printf("[harness] SpawnTmuxWindow: name=%q prev=%s\n[harness] shell: %s\n", windowName, prev, shellCmd)
	if err := exec.Command("tmux", "new-window", "-n", windowName, "sh", "-c", shellCmd).Run(); err != nil {
		return fmt.Errorf("tmux new-window: %w", err)
	}
	return nil
}

// SpawnTmuxPaneResume opens a new tmux pane with the kiro-cli session picker in dir.
func (k Kiro) SpawnTmuxPaneResume(dir string) error {
	shellCmd := fmt.Sprintf("cd %q && kiro-cli chat", dir)
	return exec.Command("tmux", "split-window", "-h", shellCmd).Run()
}

// Get returns the Harness for the given name. Returns Kiro for any unrecognized name.
func Get(name string) Harness {
	switch name {
	case "kiro":
		return Kiro{}
	default:
		return Kiro{}
	}
}

// JoinIDs joins a slice of session IDs into the comma-separated string used by
// the notify callback.
func JoinIDs(ids []string) string { return strings.Join(ids, ",") }
