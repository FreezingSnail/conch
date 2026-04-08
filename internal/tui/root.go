package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Root holds a stack of models. The top of the stack is the active view.
type Root struct {
	stack []tea.Model
}

func New() Root {
	return Root{stack: []tea.Model{newMenu()}}
}

func (r Root) Init() tea.Cmd {
	return r.top().Init()
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pushMsg:
		r.stack = append(r.stack, msg.model)
		return r, msg.model.Init()
	case popMsg:
		if len(r.stack) > 1 {
			r.stack = r.stack[:len(r.stack)-1]
		} else {
			return r, tea.Quit
		}
		return r, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return r, tea.Quit
		}
	}
	updated, cmd := r.top().Update(msg)
	r.stack[len(r.stack)-1] = updated
	return r, cmd
}

func (r Root) View() string {
	return r.top().View()
}

func (r Root) top() tea.Model {
	return r.stack[len(r.stack)-1]
}

// pushMsg pushes a new view onto the stack.
type pushMsg struct{ model tea.Model }

// popMsg pops the current view.
type popMsg struct{}

func push(m tea.Model) tea.Cmd {
	return func() tea.Msg { return pushMsg{model: m} }
}

func pop() tea.Cmd {
	return func() tea.Msg { return popMsg{} }
}
