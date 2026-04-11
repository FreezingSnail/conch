package tui

import (
	"fmt"
	"strings"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	tea "github.com/charmbracelet/bubbletea"
)

// feedbackTab identifies which tab is active in the feedback view.
type feedbackTab int

const (
	feedbackTabActive   feedbackTab = 0 // tickets with unaddressed notes or no notes yet
	feedbackTabArchived feedbackTab = 1 // tickets where all notes are addressed (at least one exists)
)

var feedbackTabNames = []string{"Active", "Archived"}

// feedbackView lists work items grouped into Active and Archived tabs.
// Active = tickets with at least one unaddressed note OR no notes yet.
// Archived = tickets where every note is addressed (and at least one exists).
type feedbackView struct {
	tab           feedbackTab
	tickets       []db.Ticket
	notesByTicket map[int64][]db.FeedbackNote
	cursor        int
	loaded        bool
	status        string
	w, h          int
}

// feedbackLoadedMsg carries the result of the initial data fetch.
type feedbackLoadedMsg struct {
	tickets       []db.Ticket
	notesByTicket map[int64][]db.FeedbackNote
	err           string
}

func newFeedbackView() feedbackView { return feedbackView{} }

// Title implements Titler; used by the tab bar chrome.
func (v feedbackView) Title() string { return "Feedback" }

// HelpLine implements Helper; returns context-sensitive keybinding hints.
func (v feedbackView) HelpLine() string {
	return "tab switch tabs  ↑/↓ navigate  enter diff  b burrow  r replan  esc back"
}

// Init kicks off the data load.
func (v feedbackView) Init() tea.Cmd { return loadFeedback }

// loadFeedback fetches all tickets and their feedback notes from the daemon.
func loadFeedback() tea.Msg {
	tr, err := client.Send(client.Request{Action: "list_tickets"})
	if err != nil {
		return feedbackLoadedMsg{err: err.Error()}
	}
	notes := make(map[int64][]db.FeedbackNote, len(tr.Tickets))
	for _, t := range tr.Tickets {
		nr, err := client.Send(client.Request{Action: "list_feedback_notes", TicketID: t.ID})
		if err == nil && nr.OK {
			notes[t.ID] = nr.FeedbackNotes
		}
	}
	return feedbackLoadedMsg{tickets: tr.Tickets, notesByTicket: notes}
}

// tabTickets returns the subset of tickets that belong to the active tab.
func (v feedbackView) tabTickets() []db.Ticket {
	var out []db.Ticket
	for _, t := range v.tickets {
		notes := v.notesByTicket[t.ID]
		switch v.tab {
		case feedbackTabActive:
			// Active: no notes yet, or at least one unaddressed note.
			if len(notes) == 0 || hasUnaddressed(notes) {
				out = append(out, t)
			}
		case feedbackTabArchived:
			// Archived: at least one note and all are addressed.
			if len(notes) > 0 && !hasUnaddressed(notes) {
				out = append(out, t)
			}
		}
	}
	return out
}

// hasUnaddressed reports whether any note in the slice is not yet addressed.
func hasUnaddressed(notes []db.FeedbackNote) bool {
	for _, n := range notes {
		if !n.Addressed {
			return true
		}
	}
	return false
}

// allAddressed reports whether every note in the slice is addressed.
func allAddressed(notes []db.FeedbackNote) bool {
	return len(notes) > 0 && !hasUnaddressed(notes)
}

func (v feedbackView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.w, v.h = msg.Width, msg.Height

	case feedbackLoadedMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
		}
		v.tickets = msg.tickets
		v.notesByTicket = msg.notesByTicket
		v.loaded = true
		v.cursor = 0

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return v, pop()
		case "tab":
			v.tab = (v.tab + 1) % feedbackTab(len(feedbackTabNames))
			v.cursor = 0
		case "shift+tab":
			v.tab = (v.tab + feedbackTab(len(feedbackTabNames)) - 1) % feedbackTab(len(feedbackTabNames))
			v.cursor = 0
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
			visible := v.tabTickets()
			if len(visible) == 0 || v.cursor >= len(visible) {
				return v, nil
			}
			return v, push(newDiffView(visible[v.cursor]))
		case "b":
			return v, push(newBurrowView())
		case "r":
			// Send replan_ticket to the daemon; handler wired in a later task.
			visible := v.tabTickets()
			if len(visible) == 0 || v.cursor >= len(visible) {
				return v, nil
			}
			ticketID := visible[v.cursor].ID
			return v, func() tea.Msg {
				resp, err := client.Send(client.Request{Action: "replan_ticket", TicketID: ticketID})
				if err != nil {
					return feedbackStatusMsg{"error: " + err.Error()}
				}
				if !resp.OK {
					return feedbackStatusMsg{"error: " + resp.Error}
				}
				return feedbackStatusMsg{"replan requested"}
			}
		}
	case feedbackStatusMsg:
		v.status = msg.text
	}
	return v, nil
}

// feedbackStatusMsg carries a one-line status update from an async action.
type feedbackStatusMsg struct{ text string }

func (v feedbackView) View() string {
	var b strings.Builder

	// Tab bar.
	for i, name := range feedbackTabNames {
		if feedbackTab(i) == v.tab {
			b.WriteString(StyleActiveTab.Render(" [" + name + "] "))
		} else {
			b.WriteString(StyleInactiveTab.Render("  " + name + "  "))
		}
	}
	b.WriteString("\n\n")

	if !v.loaded {
		b.WriteString("  loading...\n")
		return b.String()
	}

	visible := v.tabTickets()
	if len(visible) == 0 {
		b.WriteString("  nothing here yet\n")
	} else {
		for i, t := range visible {
			notes := v.notesByTicket[t.ID]
			noteCount := len(notes)

			// Build the display label.
			title := t.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}
			num := t.TicketNumber
			if num == "" {
				num = fmt.Sprintf("#%d", t.ID)
			}

			badge := ""
			if allAddressed(notes) {
				badge = StyleSuccess.Render(" [reviewed]")
			}

			line := fmt.Sprintf("%-12s  %-40s  notes:%-3d%s", num, title, noteCount, badge)
			if i == v.cursor {
				b.WriteString(StyleCursor.Render("> "+line) + "\n")
			} else {
				b.WriteString("  " + line + "\n")
			}
		}
	}

	if v.status != "" {
		b.WriteString("\n")
		if strings.HasPrefix(v.status, "error") {
			b.WriteString("  " + StyleError.Render(v.status) + "\n")
		} else {
			b.WriteString("  " + StyleSuccess.Render(v.status) + "\n")
		}
	}

	return b.String()
}
