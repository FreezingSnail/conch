package tui

import "os/exec"

func kiroCmd() *exec.Cmd {
	return exec.Command("kiro")
}
