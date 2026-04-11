package tui

import (
	"fmt"
	"path/filepath"
	"time"

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
	logLines   []string // session log lines for selected ticket
	cursor     int
	logScroll  int
	tab        burrowTab
	status     string
	loaded     bool
	confirming bool
	confirmID  int64
	viewingLog bool
	polling    bool // true while a logPollCmd loop is active
}

type burrowLoadedMsg struct {
	tickets    []db.Ticket
	sessions   []db.Session
	taskCounts map[int64]int
	err        string
}

type burrowLogsMsg struct {
	lines []string
}

type burrowLogPollMsg struct{}

// burrowSessionsMsg is the lightweight poll result: refreshed sessions + logs.
type burrowSessionsMsg struct {
	sessions []db.Session
	lines    []string
}

// loadSessionsAndLogs fetches the current sessions list and logs for ticketID.
func loadSessionsAndLogs(ticketID int64) tea.Cmd {
	return func() tea.Msg {
		sr, err := client.Send(client.Request{Action: "list_sessions"})
		if err != nil || !sr.OK {
			return burrowSessionsMsg{}
		}
		sessionID := ticketExecSessionID(ticketID, sr.Sessions)
		var lines []string
		if sessionID != 0 {
			lr, err := client.Send(client.Request{Action: "list_session_logs", SessionID: sessionID})
			if err == nil && lr.OK {
				lines = make([]string, len(lr.SessionLogs))
				for i, l := range lr.SessionLogs {
					lines[i] = l.Payload
				}
			}
		}
		return burrowSessionsMsg{sessions: sr.Sessions, lines: lines}
	}
}

func newBurrowView() burrowView { return burrowView{} }

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

// ticketExecSessionID returns the most recent executor session ID for a ticket, or 0.
func ticketExecSessionID(ticketID int64, sessions []db.Session) int64 {
	for _, s := range sessions {
		if s.TicketID == ticketID && s.Harness == "kiro-executor" {
			return s.ID
		}
	}
	return 0
}

func loadLogsForTicket(ticketID int64, sessions []db.Session) tea.Cmd {
	sessionID := ticketExecSessionID(ticketID, sessions)
	if sessionID == 0 {
		return func() tea.Msg { return burrowLogsMsg{} }
	}
	return func() tea.Msg {
		resp, err := client.Send(client.Request{Action: "list_session_logs", SessionID: sessionID})
		if err != nil || !resp.OK {
			return burrowLogsMsg{}
		}
		lines := make([]string, len(resp.SessionLogs))
		for i, l := range resp.SessionLogs {
			lines[i] = l.Payload
		}
		return burrowLogsMsg{lines: lines}
	}
}

func logPollCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return burrowLogPollMsg{} })
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
	case burrowLoadedMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
		}
		// Preserve selected ticket across refresh.
		var prevTicketID int64
		if prev := v.tabTickets(); v.cursor < len(prev) {
			prevTicketID = prev[v.cursor].ID
		}
		v.tickets = msg.tickets
		v.sessions = msg.sessions
		v.taskCounts = msg.taskCounts
		v.loaded = true
		v.polling = false
		// Restore cursor to the same ticket, fall back to 0.
		v.cursor = 0
		for i, t := range v.tabTickets() {
			if t.ID == prevTicketID {
				v.cursor = i
				break
			}
		}
		if visible := v.tabTickets(); len(visible) > 0 {
			cmds := []tea.Cmd{loadLogsForTicket(visible[v.cursor].ID, v.sessions)}
			if ticketExecStatus(visible[v.cursor].ID, v.sessions) == "running" {
				v.polling = true
				cmds = append(cmds, logPollCmd())
			}
			return v, tea.Batch(cmds...)
		}

	case burrowLogsMsg:
		v.logLines = msg.lines
		// poll loop is driven solely by burrowLogPollMsg; don't reschedule here

	case burrowLogPollMsg:
		visible := v.tabTickets()
		if v.cursor >= len(visible) {
			v.polling = false
			return v, nil
		}
		// Always do a lightweight refresh so we detect status changes.
		return v, tea.Batch(loadSessionsAndLogs(visible[v.cursor].ID), logPollCmd())

	case burrowSessionsMsg:
		v.sessions = msg.sessions
		v.logLines = msg.lines
		// Stop polling once the session is no longer running.
		visible := v.tabTickets()
		if v.cursor >= len(visible) || ticketExecStatus(visible[v.cursor].ID, v.sessions) != "running" {
			v.polling = false
		}

	case burrowStartedMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
		} else {
			v.status = fmt.Sprintf("executor started (session %d) — r to refresh", msg.sessionID)
			v.loaded = false
			return v, loadBurrow
		}

	case tea.KeyMsg:
		if v.viewingLog {
			switch msg.String() {
			case "esc", "q", "l":
				v.viewingLog = false
			case "up", "k":
				if v.logScroll > 0 {
					v.logScroll--
				}
			case "down", "j":
				if v.logScroll < len(v.logLines)-1 {
					v.logScroll++
				}
			}
			return v, nil
		}
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
			if visible := v.tabTickets(); len(visible) > 0 {
				return v, loadLogsForTicket(visible[0].ID, v.sessions)
			}
		case "shift+tab":
			v.tab = (v.tab + burrowTab(len(burrowTabNames)) - 1) % burrowTab(len(burrowTabNames))
			v.cursor = 0
			v.status = ""
			if visible := v.tabTickets(); len(visible) > 0 {
				return v, loadLogsForTicket(visible[0].ID, v.sessions)
			}
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
				visible := v.tabTickets()
				if v.cursor < len(visible) {
					cmds := []tea.Cmd{loadLogsForTicket(visible[v.cursor].ID, v.sessions)}
					if !v.polling && ticketExecStatus(visible[v.cursor].ID, v.sessions) == "running" {
						v.polling = true
						cmds = append(cmds, logPollCmd())
					}
					return v, tea.Batch(cmds...)
				}
			}
		case "down", "j":
			visible := v.tabTickets()
			if v.cursor < len(visible)-1 {
				v.cursor++
				cmds := []tea.Cmd{loadLogsForTicket(visible[v.cursor].ID, v.sessions)}
				if !v.polling && ticketExecStatus(visible[v.cursor].ID, v.sessions) == "running" {
					v.polling = true
					cmds = append(cmds, logPollCmd())
				}
				return v, tea.Batch(cmds...)
			}
		case "enter":
			return v, v.handleEnter()
		case "l":
			v.viewingLog = true
			v.logScroll = len(v.logLines) - 1
			if v.logScroll < 0 {
				v.logScroll = 0
			}
		case "x":
			visible := v.tabTickets()
			if len(visible) == 0 || v.cursor >= len(visible) {
				return v, nil
			}
			sessionID := ticketExecSessionID(visible[v.cursor].ID, v.sessions)
			if sessionID == 0 {
				return v, nil
			}
			return v, func() tea.Msg {
				resp, err := client.Send(client.Request{Action: "kill_session", SessionID: sessionID})
				if err != nil || !resp.OK {
					errMsg := "kill failed"
					if err != nil {
						errMsg = err.Error()
					} else {
						errMsg = resp.Error
					}
					return burrowStartedMsg{err: errMsg}
				}
				return loadBurrow()
			}
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
	if v.viewingLog {
		s := "  Session Log\n\n"
		if len(v.logLines) == 0 {
			s += "  (no logs)\n"
		} else {
			// show a window of lines around logScroll
			const pageSize = 30
			start := v.logScroll - pageSize/2
			if start < 0 {
				start = 0
			}
			end := start + pageSize
			if end > len(v.logLines) {
				end = len(v.logLines)
			}
			for _, line := range v.logLines[start:end] {
				s += "  " + line + "\n"
			}
			s += fmt.Sprintf("\n  line %d/%d\n", v.logScroll+1, len(v.logLines))
		}
		s += "\n  ↑/↓ scroll  l/esc back\n"
		return s
	}

	s := "  Burrow 🐌\n\n"

	// Tab bar
	for i, name := range burrowTabNames {
		if burrowTab(i) == v.tab {
			s += fmt.Sprintf("  [%s]", name)
		} else {
			s += fmt.Sprintf("   %s ", name)
		}
		if i < len(burrowTabNames)-1 {
			s += "  "
		}
	}
	s += "\n\n"

	if v.confirming {
		return s + fmt.Sprintf("  hard delete ticket #%d? [y/n]\n", v.confirmID)
	}

	if !v.loaded {
		return s + "  loading...\n"
	}

	visible := v.tabTickets()
	if len(visible) == 0 {
		s += "  nothing here yet\n"
	} else {
		s += fmt.Sprintf("  %-14s  %-28s  %-18s  %s\n", "TICKET", "DESCRIPTION", "REPO", "TASKS")
		s += fmt.Sprintf("  %-14s  %-28s  %-18s  %s\n", "------", "-----------", "----", "-----")
		for i, t := range visible {
			cursor := "  "
			if i == v.cursor {
				cursor = "> "
			}
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
			s += fmt.Sprintf("%s%-14s  %-28s  %-18s  %s\n", cursor, num, desc, repo, tasks)
		}
	}

	if v.status != "" {
		s += "\n  " + v.status + "\n"
	}

	// Session log panel
	s += "\n  ── session log ──────────────────────────────────────────\n"
	if len(v.logLines) == 0 {
		s += "  (no logs)\n"
	} else {
		// show last 10 lines
		start := len(v.logLines) - 10
		if start < 0 {
			start = 0
		}
		for _, line := range v.logLines[start:] {
			s += "  " + line + "\n"
		}
	}

	s += "\n  tab switch tabs  ↑/↓ navigate  enter start  l log  x kill  D delete  r refresh  esc back\n"
	return s
}
