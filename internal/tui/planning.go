package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/harness"
	"github.com/FreezingSnail/conch/internal/kiro"
	tea "github.com/charmbracelet/bubbletea"
)

// planStep enumerates the wizard stages in order.
type planStep int

const (
	stepTicketNum  planStep = iota // collect free-text ticket number
	stepRepoPicker                 // multi-select repos
	stepSummary                    // show task summary after kiro exits
)

// planningWizard is the multi-step planning flow model.
type planningWizard struct {
	step planStep
	w, h int // terminal dimensions

	// text input fields
	ticketNum string

	// repo picker state
	repos   []string
	repoSel []bool // parallel to repos; true = selected
	repoCur int
	loaded  bool

	// set after plan_setup succeeds
	ticketID  int64
	sessionID int64
	worktree  string // first worktree path, used as kiro working dir

	// kiro session linking: snapshot of session IDs before launch
	beforeIDs []string

	// summary state
	tasks  []taskLine
	status string
}

type taskLine struct {
	title  string
	status string
}

// reposLoadedForPlanMsg carries the repo list from the daemon.
type reposLoadedForPlanMsg struct{ repos []string }

// planSetupDoneMsg carries the result of the plan_setup daemon call.
type planSetupDoneMsg struct {
	ticketID  int64
	sessionID int64
	worktree  string
	err       string
}

// kiroExitMsg is returned by tea.ExecProcess when kiro exits.
type kiroExitMsg struct{ err error }

// planTasksMsg carries the task list fetched after kiro exits.
type planTasksMsg struct {
	tasks []taskLine
	err   string
}

func newPlanningWizard() planningWizard { return planningWizard{} }

// Title implements Titler; used by the tab bar chrome.
func (w planningWizard) Title() string { return "Plan" }

// HelpLine implements Helper; returns context-sensitive keybinding hints.
func (w planningWizard) HelpLine() string {
	switch w.step {
	case stepTicketNum:
		return "enter next  esc back"
	case stepRepoPicker:
		return "space toggle  enter confirm  esc back"
	default:
		return "any key to return to menu"
	}
}

func (w planningWizard) Init() tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Send(client.Request{Action: "list_repos"})
		if err != nil || !resp.OK {
			return reposLoadedForPlanMsg{}
		}
		return reposLoadedForPlanMsg{repos: resp.Repos}
	}
}

func (w planningWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		w.w, w.h = msg.Width, msg.Height

	case reposLoadedForPlanMsg:
		w.repos = msg.repos
		w.repoSel = make([]bool, len(msg.repos))
		w.loaded = true

	case planSetupDoneMsg:
		if msg.err != "" {
			w.status = "error: " + msg.err
			w.step = stepRepoPicker
			return w, nil
		}
		w.ticketID = msg.ticketID
		w.sessionID = msg.sessionID
		w.worktree = msg.worktree
		w.beforeIDs = kiro.ListSessionIDs(w.worktree)
		prompt := kiro.BuildPrompt(w.ticketNum, w.ticketID)
		if harness.InTmux() {
			err := harness.SpawnTmuxWindow(kiro.Kiro{}, w.ticketNum, "planning", prompt, w.worktree, w.sessionID, harness.JoinIDs(w.beforeIDs))
			if err != nil {
				w.status = "tmux error: " + err.Error()
				w.step = stepRepoPicker
				return w, nil
			}
			return w, pop()
		}
		cmd := kiro.Kiro{}.InteractiveWithAgent("planning", prompt, w.worktree)
		return w, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return kiroExitMsg{err: err}
		})

	case kiroExitMsg:
		// Diff kiro sessions to find the UUID created during this run.
		afterIDs := kiro.ListSessionIDs(w.worktree)
		if uuid := kiro.NewSessionID(w.beforeIDs, afterIDs); uuid != "" {
			client.Send(client.Request{ //nolint:errcheck
				Action:        "set_kiro_session",
				SessionID:     w.sessionID,
				KiroSessionID: uuid,
			})
		}
		client.Send(client.Request{ //nolint:errcheck
			Action:    "update_session_status",
			SessionID: w.sessionID,
			Status:    "completed",
		})
		ticketID := w.ticketID
		return w, func() tea.Msg {
			resp, err := client.Send(client.Request{Action: "list_tasks", TicketID: ticketID})
			if err != nil {
				return planTasksMsg{err: err.Error()}
			}
			if !resp.OK {
				return planTasksMsg{err: resp.Error}
			}
			lines := make([]taskLine, len(resp.Tasks))
			for i, t := range resp.Tasks {
				lines[i] = taskLine{title: t.Title, status: t.Status}
			}
			return planTasksMsg{tasks: lines}
		}

	case planTasksMsg:
		if msg.err != "" {
			w.status = "error fetching tasks: " + msg.err
		}
		w.tasks = msg.tasks
		w.step = stepSummary

	case tea.KeyMsg:
		return w.handleKey(msg)
	}
	return w, nil
}

func (w planningWizard) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch w.step {
	case stepTicketNum:
		w.ticketNum = applyTextInput(w.ticketNum, msg.String())
		if msg.String() == "enter" {
			if strings.TrimSpace(w.ticketNum) != "" {
				w.step = stepRepoPicker
			}
		} else if msg.String() == "esc" {
			return w, pop()
		}
	case stepRepoPicker:
		return w.handleRepoPicker(msg)
	case stepSummary:
		return w, pop()
	}
	return w, nil
}

// applyTextInput applies a single keystroke to a text field value and returns
// the updated value. enter and esc are handled by the caller.
func applyTextInput(field, key string) string {
	switch key {
	case "enter", "esc":
		return field
	case "backspace":
		if len(field) > 0 {
			return field[:len(field)-1]
		}
	default:
		if len(key) == 1 {
			return field + key
		}
	}
	return field
}

func (w planningWizard) handleRepoPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !w.loaded {
		return w, nil
	}
	switch msg.String() {
	case "esc":
		w.step = stepTicketNum
	case "up", "k":
		if w.repoCur > 0 {
			w.repoCur--
		}
	case "down", "j":
		if w.repoCur < len(w.repos)-1 {
			w.repoCur++
		}
	case " ":
		if len(w.repos) > 0 {
			w.repoSel[w.repoCur] = !w.repoSel[w.repoCur]
		}
	case "enter":
		selected := w.selectedRepos()
		if len(selected) == 0 {
			w.status = "select at least one repo"
			return w, nil
		}
		w.status = ""
		return w, w.doPlanSetup(selected)
	}
	return w, nil
}

// selectedRepos returns the paths of all toggled repos.
func (w planningWizard) selectedRepos() []string {
	var out []string
	for i, sel := range w.repoSel {
		if sel {
			out = append(out, w.repos[i])
		}
	}
	return out
}

// doPlanSetup sends plan_setup to the daemon and returns a Cmd.
func (w planningWizard) doPlanSetup(repos []string) tea.Cmd {
	ticketNum := w.ticketNum
	return func() tea.Msg {
		resp, err := client.Send(client.Request{
			Action:       "plan_setup",
			TicketNumber: ticketNum,
			Title:        ticketNum,
			Repos:        repos,
		})
		if err != nil {
			return planSetupDoneMsg{err: err.Error()}
		}
		if !resp.OK {
			return planSetupDoneMsg{err: resp.Error}
		}
		worktree := filepath.Join(os.Getenv("HOME"), ".conch", "worktrees", ticketNum, filepath.Base(repos[0]))
		return planSetupDoneMsg{
			ticketID:  resp.ID,
			sessionID: resp.SessionID,
			worktree:  worktree,
		}
	}
}

func (w planningWizard) View() string {
	switch w.step {
	case stepTicketNum:
		return fmt.Sprintf("  %s\n\n%s\n", StyleTitle.Render("Plan — new ticket"), wrapInput("Ticket number: ", w.ticketNum, w.w))
	case stepRepoPicker:
		return w.viewRepoPicker()
	case stepSummary:
		return w.viewSummary()
	}
	return ""
}

func (w planningWizard) viewRepoPicker() string {
	s := "  " + StyleTitle.Render(fmt.Sprintf("Plan — %s — select repos", w.ticketNum)) + "\n\n"
	if !w.loaded {
		return s + "  loading repos...\n"
	}
	if len(w.repos) == 0 {
		return s + "  no repos found\n"
	}
	for i, r := range w.repos {
		check := "[ ]"
		if w.repoSel[i] {
			check = "[x]"
		}
		row := fmt.Sprintf("%s %s", check, filepath.Base(r))
		if i == w.repoCur {
			s += StyleCursor.Render("> "+row) + "\n"
		} else {
			s += "  " + row + "\n"
		}
	}
	if w.status != "" {
		s += "\n  " + statusLine(w.status) + "\n"
	}
	return s
}

func (w planningWizard) viewSummary() string {
	s := "  " + StyleTitle.Render(fmt.Sprintf("Plan — %s — complete", w.ticketNum)) + "\n\n"
	if w.status != "" {
		s += "  " + statusLine(w.status) + "\n\n"
	}
	if len(w.tasks) == 0 {
		s += "  no tasks created yet\n"
	} else {
		s += fmt.Sprintf("  %d task(s) created:\n\n", len(w.tasks))
		for _, t := range w.tasks {
			s += fmt.Sprintf("  [%s] %s\n", t.status, t.title)
		}
	}
	return s
}
