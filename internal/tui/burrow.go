package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	tea "github.com/charmbracelet/bubbletea"
)

type burrowTab int

const (
	burrowTabPending     burrowTab = iota // unstarted + running
	burrowTabNeedsReview                  // session completed, awaiting review
	burrowTabComplete                     // reviewed (future)
)

var burrowTabNames = []string{"Pending", "Needs Review", "Complete"}

type burrowView struct {
	tickets    []db.Ticket
	sessions   []db.Session // all sessions, used to derive ticket state
	taskCounts map[int64]int
	cursor     int
	tab        burrowTab
	status     string
	loaded     bool
	confirming bool
	confirmID  int64
	w, h       int
}

type burrowLoadedMsg struct {
	tickets    []db.Ticket
	sessions   []db.Session
	taskCounts map[int64]int
	err        string
}

func newBurrowView() burrowView { return burrowView{} }

// Title implements Titler; used by the tab bar chrome.
func (v burrowView) Title() string { return "Burrow" }

// HelpLine implements Helper; returns context-sensitive keybinding hints.
func (v burrowView) HelpLine() string {
	return "tab switch tabs  ↑/↓ navigate  enter start  D delete  r refresh  esc back"
}

func (v burrowView) Init() tea.Cmd { return loadBurrow }

func loadBurrow() tea.Msg {
	tr, err := client.Send(client.Request{Action: "list_tickets"})
	if err != nil {
		return burrowLoadedMsg{err: err.Error()}
	}
	sr, err := client.Send(client.Request{Action: "list_sessions"})
	if err != nil {
		return burrowLoadedMsg{err: err.Error()}
	}
	counts := make(map[int64]int, len(tr.Tickets))
	for _, t := range tr.Tickets {
		resp, err := client.Send(client.Request{Action: "list_tasks", TicketID: t.ID})
		if err == nil && resp.OK {
			counts[t.ID] = len(resp.Tasks)
		}
	}
	return burrowLoadedMsg{tickets: tr.Tickets, sessions: sr.Sessions, taskCounts: counts}
}

// ticketExecStatus returns the most recent executor session status for a ticket.
// Returns "" if no executor session exists.
func ticketExecStatus(ticketID int64, sessions []db.Session) string {
	for _, s := range sessions {
		if s.TicketID == ticketID && s.Harness == "kiro-executor" {
			return s.Status // sessions are ordered newest-first
		}
	}
	return ""
}

func (v burrowView) tabTickets() []db.Ticket {
	var out []db.Ticket
	for _, t := range v.tickets {
		st := ticketExecStatus(t.ID, v.sessions)
		switch v.tab {
		case burrowTabPending:
			if st == "" || st == "running" {
				out = append(out, t)
			}
		case burrowTabNeedsReview:
			if st == "completed" || st == "error" {
				out = append(out, t)
			}
		case burrowTabComplete:
			// future
		}
	}
	return out
}

func (v burrowView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.w, v.h = msg.Width, msg.Height

	case burrowLoadedMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
		}
		v.tickets = msg.tickets
		v.sessions = msg.sessions
		v.taskCounts = msg.taskCounts
		v.loaded = true
		v.cursor = 0

	case burrowStartedMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
		} else {
			v.status = fmt.Sprintf("executor started (session %d) — r to refresh", msg.sessionID)
			v.loaded = false
			return v, loadBurrow
		}

	case tea.KeyMsg:
		if v.confirming {
			switch msg.String() {
			case "y", "Y":
				v.confirming = false
				id := v.confirmID
				return v, func() tea.Msg {
					resp, err := client.Send(client.Request{Action: "delete_ticket", TicketID: id})
					if err != nil {
						return burrowStartedMsg{err: err.Error()}
					}
					if !resp.OK {
						return burrowStartedMsg{err: resp.Error}
					}
					return loadBurrow()
				}
			default:
				v.confirming = false
			}
			return v, nil
		}
		switch msg.String() {
		case "esc", "q":
			return v, pop()
		case "r":
			v.loaded = false
			v.status = ""
			return v, loadBurrow
		case "tab":
			v.tab = (v.tab + 1) % burrowTab(len(burrowTabNames))
			v.cursor = 0
			v.status = ""
		case "shift+tab":
			v.tab = (v.tab + burrowTab(len(burrowTabNames)) - 1) % burrowTab(len(burrowTabNames))
			v.cursor = 0
			v.status = ""
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			visible := v.tabTickets()
			if v.cursor < len(visible)-1 {
				v.cursor++
			}
		case "enter":
			return v, v.handleEnter()
		case "D":
			visible := v.tabTickets()
			if len(visible) > 0 && v.cursor < len(visible) {
				v.confirming = true
				v.confirmID = visible[v.cursor].ID
			}
		}
	}
	return v, nil
}

func (v burrowView) handleEnter() tea.Cmd {
	visible := v.tabTickets()
	if len(visible) == 0 || v.cursor >= len(visible) {
		return nil
	}
	t := visible[v.cursor]

	if v.tab == burrowTabNeedsReview {
		return func() tea.Msg { return burrowStartedMsg{err: "review flow coming soon"} }
	}

	if v.tab != burrowTabPending {
		return nil
	}

	st := ticketExecStatus(t.ID, v.sessions)
	if st == "running" {
		return func() tea.Msg { return burrowStartedMsg{err: "executor already running for this ticket"} }
	}
	if t.WorktreePath == "" {
		return func() tea.Msg { return burrowStartedMsg{err: "no worktree — create one first"} }
	}

	ticketID := t.ID
	return func() tea.Msg {
		resp, err := client.Send(client.Request{Action: "exec_start", TicketID: ticketID})
		if err != nil {
			return burrowStartedMsg{err: err.Error()}
		}
		if !resp.OK {
			return burrowStartedMsg{err: resp.Error}
		}
		return burrowStartedMsg{sessionID: resp.ID}
	}
}

type burrowStartedMsg struct {
	sessionID int64
	err       string
}

func (v burrowView) View() string {
	var b strings.Builder

	// Tab bar: active tab uses accent colour, inactive tabs are dimmed.
	for i, name := range burrowTabNames {
		if burrowTab(i) == v.tab {
			b.WriteString(StyleActiveTab.Render(" [" + name + "] "))
		} else {
			b.WriteString(StyleInactiveTab.Render("  " + name + "  "))
		}
	}
	b.WriteString("\n\n")

	if v.confirming {
		b.WriteString(StyleError.Render(fmt.Sprintf("  hard delete ticket #%d? [y/n]", v.confirmID)) + "\n")
		return b.String()
	}

	if !v.loaded {
		b.WriteString("  loading...\n")
		return b.String()
	}

	visible := v.tabTickets()
	if len(visible) == 0 {
		b.WriteString("  nothing here yet\n")
	} else {
		// Header row in bold.
		header := fmt.Sprintf("  %-14s  %-28s  %-18s  %s", "TICKET", "DESCRIPTION", "REPO", "TASKS")
		b.WriteString(StyleTitle.Render(header) + "\n")
		sep := fmt.Sprintf("  %-14s  %-28s  %-18s  %s", "------", "-----------", "----", "-----")
		b.WriteString(StyleTitle.Render(sep) + "\n")

		for i, t := range visible {
			num := t.TicketNumber
			if num == "" {
				num = fmt.Sprintf("#%d", t.ID)
			}
			desc := t.Title
			if len(desc) > 28 {
				desc = desc[:25] + "..."
			}
			repo := filepath.Base(t.Repo)
			if repo == "." || repo == "" {
				repo = "-"
			}
			tasks := fmt.Sprintf("%d", v.taskCounts[t.ID])
			st := ticketExecStatus(t.ID, v.sessions)
			if st == "running" {
				tasks += " (running)"
			} else if st == "error" {
				tasks += " (error)"
			}
			row := fmt.Sprintf("> %-14s  %-28s  %-18s  %s", num, desc, repo, tasks)
			plain := fmt.Sprintf("  %-14s  %-28s  %-18s  %s", num, desc, repo, tasks)
			if i == v.cursor {
				b.WriteString(StyleCursor.Render(row) + "\n")
			} else {
				b.WriteString(plain + "\n")
			}
		}
	}

	if v.status != "" {
		b.WriteString("\n")
		// Distinguish error vs success status by prefix convention.
		if len(v.status) >= 5 && v.status[:5] == "error" {
			b.WriteString("  " + StyleError.Render(v.status) + "\n")
		} else {
			b.WriteString("  " + StyleSuccess.Render(v.status) + "\n")
		}
	}
	return b.String()
}
