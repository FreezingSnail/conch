package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

var menuItems = []string{"Plan", "Execute", "Tickets", "New Ticket", "Worktrees", "Sessions", "Planning Sessions"}

type menu struct {
	cursor int
}

func newMenu() menu { return menu{} }

func (m menu) Init() tea.Cmd { return nil }

func (m menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
			return m, push(newView(menuItems[m.cursor]))
		case "q", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m menu) View() string {
	s := "  conch\n\n"
	for i, item := range menuItems {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		s += fmt.Sprintf("%s%s\n", cursor, item)
	}
	s += "\n  ↑/↓ navigate  enter select  q quit\n"
	return s
}
