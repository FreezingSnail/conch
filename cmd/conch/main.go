package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "notify":
			runNotify(os.Args[2:])
			return
		case "escalate":
			runEscalate(os.Args[2:])
			return
		case "task":
			runTask(os.Args[2:])
			return
		}
	}
	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func runNotify(args []string) {
	fs := flag.NewFlagSet("notify", flag.ExitOnError)
	sessionID := fs.Int64("session-id", 0, "conch session ID")
	worktree := fs.String("worktree", "", "worktree path")
	fs.Parse(args) //nolint:errcheck

	if *sessionID == 0 || *worktree == "" {
		fmt.Fprintln(os.Stderr, "notify: --session-id and --worktree required")
		os.Exit(1)
	}
	resp, err := client.Send(client.Request{
		Action:    "plan_complete",
		SessionID: *sessionID,
		Worktree:  *worktree,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "notify:", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintln(os.Stderr, "notify:", resp.Error)
		os.Exit(1)
	}
}

func runEscalate(args []string) {
	fs := flag.NewFlagSet("escalate", flag.ExitOnError)
	sessionID := fs.Int64("session-id", 0, "conch session ID to escalate")
	fs.Parse(args) //nolint:errcheck
	if *sessionID == 0 {
		fmt.Fprintln(os.Stderr, "escalate: --session-id required")
		os.Exit(1)
	}
	resp, err := client.Send(client.Request{
		Action:    "update_session_status",
		SessionID: *sessionID,
		Status:    "needs_human",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "escalate:", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintln(os.Stderr, "escalate:", resp.Error)
		os.Exit(1)
	}
}
