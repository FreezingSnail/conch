package tui

import (
	"fmt"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/harness"
	tea "github.com/charmbracelet/bubbletea"
)

type planningSessionsView struct {
	sessions []db.Session
	tickets  map[int64]db.Ticket // keyed by ticket ID
	cursor   int
	status   string
	loaded   bool
}

type planningSessionsLoadedMsg struct {
	sessions []db.Session
	tickets  map[int64]db.Ticket
	err      string
}

func newPlanningSessionsView() planningSessionsView { return planningSessionsView{} }

func (v planningSessionsView) Init() tea.Cmd { return loadPlanningSessions }

func loadPlanningSessions() tea.Msg {
	sr, err := client.Send(client.Request{Action: "list_sessions"})
	if err != nil {
		return planningSessionsLoadedMsg{err: err.Error()}
	}
	tr, err := client.Send(client.Request{Action: "list_tickets"})
	if err != nil {
		return planningSessionsLoadedMsg{err: err.Error()}
	}
	ticketMap := make(map[int64]db.Ticket, len(tr.Tickets))
	for _, t := range tr.Tickets {
		ticketMap[t.ID] = t
	}
	// Keep only sessions linked to a ticket (planning sessions).
	var planning []db.Session
	for _, s := range sr.Sessions {
		if s.TicketID > 0 {
			planning = append(planning, s)
		}
	}
	return planningSessionsLoadedMsg{sessions: planning, tickets: ticketMap}
}

func (v planningSessionsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case planningSessionsLoadedMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
		}
		v.sessions = msg.sessions
		v.tickets = msg.tickets
		v.loaded = true

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return v, pop()
		case "r":
			v.loaded = false
			return v, loadPlanningSessions
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(v.sessions)-1 {
				v.cursor++
			}
		case "enter":
			if len(v.sessions) == 0 {
				return v, nil
			}
			s := v.sessions[v.cursor]
			t := v.tickets[s.TicketID]
			if t.WorktreePath == "" {
				v.status = "no worktree for this session"
				return v, nil
			}
			if harness.InTmux() {
				k := harness.Kiro{}
				if err := k.SpawnTmuxPaneResume(t.WorktreePath); err != nil {
					v.status = "tmux error: " + err.Error()
				}
				return v, nil
			}
			// Fallback: take over terminal with kiro session picker.
			k := harness.Kiro{}
			cmd := k.Interactive()
			cmd.Dir = t.WorktreePath
			return v, tea.ExecProcess(cmd, func(err error) tea.Msg { return nil })
		}
	}
	return v, nil
}

func (v planningSessionsView) View() string {
	s := "  Planning Sessions\n\n"
	if !v.loaded {
		return s + "  loading...\n"
	}
	if len(v.sessions) == 0 {
		return s + "  no planning sessions yet\n\n  r refresh  esc back\n"
	}
	for i, sess := range v.sessions {
		cursor := "  "
		if i == v.cursor {
			cursor = "> "
		}
		title := fmt.Sprintf("session %d", sess.ID)
		if t, ok := v.tickets[sess.TicketID]; ok {
			title = t.TicketNumber
			if t.Title != "" {
				title += " " + t.Title
			}
		}
		s += fmt.Sprintf("%s[%s] %s  %s\n", cursor, sess.Status, title, sess.StartedAt.Format("01-02 15:04"))
	}
	if v.status != "" {
		s += "\n  " + v.status + "\n"
	}
	s += "\n  ↑/↓ navigate  enter resume  r refresh  esc back\n"
	return s
}
