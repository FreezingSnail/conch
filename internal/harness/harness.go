package harness

import (
	"os/exec"
)

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

func Get(name string) Harness {
	switch name {
	case "kiro":
		return Kiro{}
	default:
		return Kiro{}
	}
}
