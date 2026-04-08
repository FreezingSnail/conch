package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	tea "github.com/charmbracelet/bubbletea"
)

type worktreesView struct {
	tickets []db.Ticket
	cursor  int
	loaded  bool
	status  string
	// confirm state
	confirming  bool
	confirmMsg  string
	confirmAction string
	// output overlay
	output string
}

type worktreesLoadedMsg struct{ tickets []db.Ticket }

func newWorktreesView() worktreesView { return worktreesView{} }

func (v worktreesView) Init() tea.Cmd {
	return v.load()
}

func (v worktreesView) load() tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Send(client.Request{Action: "list_worktrees"})
		if err != nil || !resp.OK {
			return worktreesLoadedMsg{}
		}
		return worktreesLoadedMsg{tickets: resp.Tickets}
	}
}

func (v worktreesView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case worktreesLoadedMsg:
		v.tickets = msg.tickets
		v.loaded = true

	case tea.KeyMsg:
		// dismiss output overlay
		if v.output != "" {
			v.output = ""
			return v, nil
		}

		// confirmation prompt
		if v.confirming {
			switch msg.String() {
			case "y", "Y":
				v.confirming = false
				return v, v.sendAction(v.confirmAction)
			default:
				v.confirming = false
				v.confirmMsg = ""
				v.confirmAction = ""
			}
			return v, nil
		}

		if !v.loaded {
			return v, nil
		}

		switch msg.String() {
		case "esc", "q":
			return v, pop()
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(v.tickets)-1 {
				v.cursor++
			}
		case "r":
			v.loaded = false
			return v, v.load()
		case "s":
			return v, v.sendAction("worktree_status")
		case "d":
			return v, v.sendAction("worktree_diff")
		case "p":
			return v, v.sendAction("push_worktree")
		case "R":
			return v, v.sendAction("sync_worktrees")
		case "P":
			v.confirming = true
			v.confirmMsg = "open PR for this worktree? [y/n]"
			v.confirmAction = "open_pr"
		case "x":
			v.confirming = true
			v.confirmMsg = "delete this worktree? [y/n]"
			v.confirmAction = "remove_worktree"
		case "S":
			v.status = "syncing..."
			return v, v.sendGlobal("sync_worktrees")
		}
	case worktreeActionMsg:
		if msg.lines != "" {
			v.output = msg.lines
		} else if msg.err != "" {
			v.status = "error: " + msg.err
		} else {
			v.status = msg.ok
		}
		if msg.reload {
			v.loaded = false
			return v, v.load()
		}
	}
	return v, nil
}

type worktreeActionMsg struct {
	ok     string
	err    string
	lines  string
	reload bool
}

func (v worktreesView) currentID() int64 {
	if len(v.tickets) == 0 {
		return 0
	}
	return v.tickets[v.cursor].ID
}

func (v worktreesView) sendAction(action string) tea.Cmd {
	id := v.currentID()
	return func() tea.Msg {
		resp, err := client.Send(client.Request{Action: action, TicketID: id})
		if err != nil {
			return worktreeActionMsg{err: err.Error()}
		}
		if !resp.OK {
			return worktreeActionMsg{err: resp.Error}
		}
		reload := action == "remove_worktree"
		if len(resp.Lines) > 0 {
			return worktreeActionMsg{lines: strings.Join(resp.Lines, "\n"), reload: reload}
		}
		return worktreeActionMsg{ok: action + " ok", reload: reload}
	}
}

func (v worktreesView) sendGlobal(action string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Send(client.Request{Action: action})
		if err != nil {
			return worktreeActionMsg{err: err.Error()}
		}
		if !resp.OK {
			return worktreeActionMsg{err: resp.Error}
		}
		return worktreeActionMsg{ok: "sync complete", reload: true}
	}
}

func (v worktreesView) View() string {
	if v.output != "" {
		lines := strings.Split(v.output, "\n")
		if len(lines) > 30 {
			lines = lines[:30]
		}
		s := "  Output\n\n"
		for _, l := range lines {
			s += "  " + l + "\n"
		}
		s += "\n  any key to dismiss\n"
		return s
	}

	if v.confirming {
		return fmt.Sprintf("  Worktrees\n\n  %s\n", v.confirmMsg)
	}

	if !v.loaded {
		return "  Worktrees\n\n  loading...\n"
	}

	s := "  Worktrees\n\n"
	if len(v.tickets) == 0 {
		s += "  no active worktrees\n"
	} else {
		for i, t := range v.tickets {
			cursor := "  "
			if i == v.cursor {
				cursor = "> "
			}
			s += fmt.Sprintf("%s[%d] %s  %s\n", cursor, t.ID, t.Title, filepath.Base(t.WorktreePath))
		}
	}
	if v.status != "" {
		s += "\n  " + v.status + "\n"
	}
	s += "\n  s status  d diff  p push  R rebase  P open-pr  x delete  S sync-all  r refresh  esc back\n"
	return s
}
