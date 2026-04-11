package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/FreezingSnail/conch/internal/client"
	tea "github.com/charmbracelet/bubbletea"
)

// newView routes to the correct view by name.
func newView(name string) tea.Model {
	switch name {
	case "Plan":
		return newPlanningWizard()
	case "Mantle":
		return newMantleView()
	case "Burrow":
		return newBurrowView()
	case "Execute":
		return newExecuteView()
	case "Tickets":
		return newListView("Tickets")
	case "New Ticket":
		return newRepoPickerView()
	case "Worktrees":
		return newWorktreesView()
	case "Sessions":
		return newListView("Sessions")
	case "Planning Sessions":
		return newPlanningSessionsView()
	case "Feedback":
		return newFeedbackView()
	default:
		return stubView{name: name}
	}
}

// --- stub fallback ---

type stubView struct{ name string }

func (s stubView) Init() tea.Cmd { return nil }
func (s stubView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && (k.String() == "esc" || k.String() == "q") {
		return s, pop()
	}
	return s, nil
}

// Title implements Titler.
func (s stubView) Title() string { return s.name }

// HelpLine implements Helper.
func (s stubView) HelpLine() string { return "esc back" }

func (s stubView) View() string {
	return StyleTitle.Render("  "+s.name) + "\n\n  (stub — press esc to go back)\n"
}

// --- execute view ---

type executeView struct {
	input   string
	status  string
	polling bool
	w, h    int
}

type pollMsg struct{}

func newExecuteView() executeView { return executeView{} }

func (e executeView) Init() tea.Cmd { return nil }

// Title implements Titler.
func (e executeView) Title() string { return "Execute" }

// HelpLine implements Helper.
func (e executeView) HelpLine() string { return "enter to submit  esc to go back" }

func (e executeView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.w, e.h = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return e, pop()
		case "enter":
			if strings.TrimSpace(e.input) == "" {
				return e, nil
			}
			resp, err := client.Send(client.Request{Action: "execute", Prompt: e.input})
			if err != nil {
				e.status = "error: " + err.Error()
				return e, nil
			}
			if !resp.OK {
				e.status = "error: " + resp.Error
				return e, nil
			}
			e.status = fmt.Sprintf("session %d started", resp.ID)
			e.polling = true
			return e, pollCmd()
		case "backspace":
			if len(e.input) > 0 {
				e.input = e.input[:len(e.input)-1]
			}
		default:
			if len(msg.String()) == 1 {
				e.input += msg.String()
			}
		}
	case pollMsg:
		resp, err := client.Send(client.Request{Action: "list_sessions"})
		if err == nil && resp.OK && len(resp.Sessions) > 0 {
			latest := resp.Sessions[0]
			e.status = fmt.Sprintf("session %d: %s", latest.ID, latest.Status)
			if latest.Status == "running" {
				return e, pollCmd()
			}
		}
		e.polling = false
	}
	return e, nil
}

func (e executeView) View() string {
	s := wrapInput("Prompt: ", e.input, e.w) + "\n"
	if e.status != "" {
		s += "\n" + statusLine(e.status)
	}
	return s
}

func pollCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return pollMsg{} })
}

// --- list views (tickets + sessions) ---

type listView struct {
	name   string
	lines  []string
	loaded bool
	w, h   int
}

func newListView(name string) listView { return listView{name: name} }

// Title implements Titler.
func (l listView) Title() string { return l.name }

// HelpLine implements Helper.
func (l listView) HelpLine() string { return "r refresh  esc back" }

func (l listView) Init() tea.Cmd {
	return func() tea.Msg { return loadListMsg{} }
}

type loadListMsg struct{}

func (l listView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case loadListMsg:
		l.lines = l.fetch()
		l.loaded = true
	case tea.WindowSizeMsg:
		sz := msg.(tea.WindowSizeMsg)
		l.w, l.h = sz.Width, sz.Height
	case tea.KeyMsg:
		k := msg.(tea.KeyMsg)
		if k.String() == "esc" || k.String() == "q" {
			return l, pop()
		}
		if k.String() == "r" {
			l.loaded = false
			return l, l.Init()
		}
	}
	return l, nil
}

func (l listView) fetch() []string {
	switch l.name {
	case "Sessions":
		resp, err := client.Send(client.Request{Action: "list_sessions"})
		if err != nil {
			return []string{"error: " + err.Error()}
		}
		if len(resp.Sessions) == 0 {
			return []string{"no sessions yet"}
		}
		var lines []string
		for _, s := range resp.Sessions {
			lines = append(lines, fmt.Sprintf("[%d] %s  status:%s  started:%s",
				s.ID, s.Harness, s.Status, s.StartedAt.Format("2006-01-02 15:04:05")))
		}
		return lines
	case "Tickets":
		resp, err := client.Send(client.Request{Action: "list_tickets"})
		if err != nil {
			return []string{"error: " + err.Error()}
		}
		if len(resp.Tickets) == 0 {
			return []string{"no tickets yet"}
		}
		var lines []string
		for _, t := range resp.Tickets {
			lines = append(lines, fmt.Sprintf("[%d] %s  status:%s", t.ID, t.Title, t.Status))
		}
		return lines
	}
	return nil
}

func (l listView) View() string {
	if !l.loaded {
		return "  loading...\n"
	}
	return RenderList(l.lines, -1, l.w) + "\n"
}
