package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type menuItem struct {
	label    string
	shortcut byte
}

var menuItems = []menuItem{
	{"Plan", 'p'},
	{"Burrow", 'b'},
	{"Tickets", 't'},
	{"New Ticket", 'n'},
	{"Worktrees", 'w'},
	{"Sessions", 's'},
	{"Planning Sessions", 'l'},
	{"Mantle", 'm'},
	{"Feedback", 'f'},
	{"Review", 'v'},
}

type menu struct {
	cursor int
	w, h   int
}

func newMenu() menu { return menu{} }

func (m menu) Title() string { return "conch" }

func (m menu) HelpLine() string {
	return "↑/↓ navigate  enter select  shortcut jump  ctrl+t new tab  q quit"
}

func (m menu) Init() tea.Cmd { return nil }

func (m menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(menuItems)-1 {
				m.cursor++
			}
		case "enter", " ":
			return m, push(newView(menuItems[m.cursor].label))
		case "q", "esc":
			return m, tea.Quit
		default:
			if len(msg.String()) == 1 {
				ch := msg.String()[0]
				for _, item := range menuItems {
					if item.shortcut == ch {
						return m, push(newView(item.label))
					}
				}
			}
		}
	}
	return m, nil
}

func (m menu) View() string {
	header := StyleTitle.Render("conch 🐌")
	s := header + "\n\n"
	for i, item := range menuItems {
		line := fmt.Sprintf("[%c] %s", item.shortcut, item.label)
		if i == m.cursor {
			s += StyleCursor.Render("> "+line) + "\n"
		} else {
			s += "    " + line + "\n"
		}
	}
	return s
}
