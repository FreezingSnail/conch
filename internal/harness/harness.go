// Package harness abstracts kiro-cli invocation for interactive and background sessions.
package harness

import (
	"os/exec"
)

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

// Get returns the Harness for the given name. Returns Kiro for any unrecognized name.
func Get(name string) Harness {
	switch name {
	case "kiro":
		return Kiro{}
	default:
		return Kiro{}
	}
}
