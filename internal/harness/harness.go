// Package harness abstracts CLI harness invocation (kiro, and future harnesses).
package harness

import (
	"os/exec"
)

// Harness abstracts a CLI agent that can run interactively or in the background.
// Interactive returns a command that takes over the terminal.
// Background returns a command that runs headlessly with the prompt as an argument.
type Harness interface {
	Name() string
	Interactive() *exec.Cmd
	Background(prompt string) *exec.Cmd
}

type Kiro struct{}

func (k Kiro) Name() string { return "kiro" }

func (k Kiro) Interactive() *exec.Cmd {
	return exec.Command("kiro")
}

func (k Kiro) Background(prompt string) *exec.Cmd {
	return exec.Command("kiro", "--prompt", prompt)
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
