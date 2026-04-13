// Package tui is the BubbleTea root model. Navigation is tab-based; each tab
// has its own push/pop stack. See tabsModel for the core logic.
package tui

import tea "github.com/charmbracelet/bubbletea"

// Root is the top-level BubbleTea model. It delegates all behaviour to
// tabsModel so that the program entry point only needs to know about Root.
type Root struct {
	tabs tabsModel
}

// New returns a Root with a single "menu" tab.
func New() Root {
	return Root{tabs: newTabsModel()}
}

func (r Root) Init() tea.Cmd { return r.tabs.Init() }

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := r.tabs.Update(msg)
	r.tabs = updated.(tabsModel)
	return r, cmd
}

func (r Root) View() string { return r.tabs.View() }

// pushMsg pushes a new view onto the active tab's stack.
type pushMsg struct{ model tea.Model }

// popMsg pops the current view from the active tab's stack.
type popMsg struct{}

// openTabMsg opens a new tab with model as its root, then dispatches followUp into it.
type openTabMsg struct {
	model    tea.Model
	followUp tea.Msg
}

// push returns a Cmd that navigates to m by pushing it onto the active stack.
func push(m tea.Model) tea.Cmd {
	return func() tea.Msg { return pushMsg{model: m} }
}

// pop returns a Cmd that navigates back by popping the current view.
func pop() tea.Cmd {
	return func() tea.Msg { return popMsg{} }
}

// openMantleDocs opens a new tab with the mantle view scrolled to the given README section.
func openMantleDocs(section string) tea.Cmd {
	return func() tea.Msg {
		return openTabMsg{
			model:    newMantleView(),
			followUp: mantleOpenDocsMsg{section: section},
		}
	}
}
