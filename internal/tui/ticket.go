package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/FreezingSnail/conch/internal/client"
	tea "github.com/charmbracelet/bubbletea"
)

// repoPickerView lets the user pick a repo then enter a ticket title.
type repoPickerView struct {
	repos    []string
	cursor   int
	loaded   bool
	selected string
	title    string
	typing   bool
	status   string
	w, h     int
}

type reposLoadedMsg struct{ repos []string }

func newRepoPickerView() repoPickerView { return repoPickerView{} }

// Title implements Titler.
func (v repoPickerView) Title() string { return "New Ticket" }

// HelpLine implements Helper; returns context-sensitive keybinding hints.
func (v repoPickerView) HelpLine() string {
	if v.typing {
		return "enter to create  esc to cancel"
	}
	return "enter select  esc back"
}

func (v repoPickerView) Init() tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Send(client.Request{Action: "list_repos"})
		if err != nil || !resp.OK {
			return reposLoadedMsg{}
		}
		return reposLoadedMsg{repos: resp.Repos}
	}
}

func (v repoPickerView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case reposLoadedMsg:
		v.repos = msg.repos
		v.loaded = true

	case tea.WindowSizeMsg:
		v.w, v.h = msg.Width, msg.Height

	case tea.KeyMsg:
		if !v.loaded {
			return v, nil
		}
		if v.typing {
			switch msg.String() {
			case "esc":
				v.typing = false
				v.selected = ""
				v.title = ""
			case "enter":
				if strings.TrimSpace(v.title) == "" {
					return v, nil
				}
				resp, err := client.Send(client.Request{
					Action: "create_ticket",
					Title:  v.title,
					Repo:   v.selected,
				})
				if err != nil {
					v.status = "error: " + err.Error()
				} else if !resp.OK {
					v.status = "error: " + resp.Error
				} else {
					v.status = fmt.Sprintf("ticket %d created", resp.ID)
					v.title = ""
					v.selected = ""
					v.typing = false
				}
			case "backspace":
				if len(v.title) > 0 {
					v.title = v.title[:len(v.title)-1]
				}
			default:
				if len(msg.String()) == 1 {
					v.title += msg.String()
				}
			}
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
			if v.cursor < len(v.repos)-1 {
				v.cursor++
			}
		case "enter":
			if len(v.repos) > 0 {
				v.selected = v.repos[v.cursor]
				v.typing = true
			}
		}
	}
	return v, nil
}

func (v repoPickerView) View() string {
	if !v.loaded {
		return "  loading repos...\n"
	}
	if len(v.repos) == 0 {
		return "  no repos found — run conch init to configure work dirs\n"
	}
	if v.typing {
		s := fmt.Sprintf("  New Ticket — %s\n\n", filepath.Base(v.selected))
		s += wrapInput("Title: ", v.title, v.w) + "\n"
		if v.status != "" {
			s += "\n" + statusLine(v.status)
		}
		return s
	}

	s := "  pick a repo\n\n"
	for i, r := range v.repos {
		row := filepath.Base(r)
		if i == v.cursor {
			s += StyleCursor.Render("> "+row) + "\n"
		} else {
			s += "  " + row + "\n"
		}
	}
	if v.status != "" {
		s += "\n" + statusLine(v.status)
	}
	return s
}

// statusLine renders a status string with StyleError or StyleSuccess based on
// whether it starts with "error".
func statusLine(s string) string {
	if strings.HasPrefix(s, "error") {
		return "  " + StyleError.Render(s) + "\n"
	}
	return "  " + StyleSuccess.Render(s) + "\n"
}
