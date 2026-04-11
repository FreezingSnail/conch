package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

var menuItems = []string{"Plan", "Burrow", "Tickets", "New Ticket", "Worktrees", "Sessions", "Planning Sessions", "Mantle"}

type menu struct {
	cursor int
	w, h   int
}

func newMenu() menu { return menu{} }

// Title satisfies Titler so tabsModel can label this tab.
func (m menu) Title() string { return "conch" }

// HelpLine satisfies Helper; tabsModel renders this in the bottom bar.
func (m menu) HelpLine() string {
	return "↑/↓ navigate  enter select  ctrl+t new tab  q quit"
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
			return m, push(newView(menuItems[m.cursor]))
		case "q", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m menu) View() string {
	// Header styled with StyleTitle; chrome and help line are handled by tabsModel.
	header := StyleTitle.Render("conch 🐌")
	return header + "\n\n" + RenderList(menuItems, m.cursor, m.w)
}
