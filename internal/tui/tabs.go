package tui

import tea "github.com/charmbracelet/bubbletea"

// Titler is implemented by views that want to supply a display name for the
// tab bar and chrome header.
type Titler interface{ Title() string }

// Helper is implemented by views that want to supply a context-specific help
// line shown at the bottom of the chrome.
type Helper interface{ HelpLine() string }

// tab is a named push/pop navigation stack.
type tab struct {
	name  string
	stack []tea.Model
}

// top returns the model at the top of the stack.
func (t *tab) top() tea.Model { return t.stack[len(t.stack)-1] }

// tabsModel manages a set of tabs, each with its own navigation stack.
// ctrl+t opens a new menu tab; ctrl+w closes the active tab; ctrl+←/→ cycles.
type tabsModel struct {
	tabs   []tab
	active int
	w, h   int
}

// newTabsModel creates a single "menu" tab as the starting state.
func newTabsModel() tabsModel {
	return tabsModel{tabs: []tab{{name: "menu", stack: []tea.Model{newMenu()}}}}
}

func (t tabsModel) Init() tea.Cmd {
	return t.tabs[t.active].top().Init()
}

func (t tabsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.w, t.h = msg.Width, msg.Height
		updated, cmd := t.tabs[t.active].top().Update(msg)
		t.tabs[t.active].stack[len(t.tabs[t.active].stack)-1] = updated
		return t, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return t, tea.Quit
		case "ctrl+t":
			t.tabs = append(t.tabs, tab{name: "menu", stack: []tea.Model{newMenu()}})
			t.active = len(t.tabs) - 1
			return t, t.tabs[t.active].top().Init()
		case "ctrl+w":
			return t.closeActive()
		case "ctrl+right":
			t.active = (t.active + 1) % len(t.tabs)
			return t, nil
		case "ctrl+left":
			t.active = (t.active - 1 + len(t.tabs)) % len(t.tabs)
			return t, nil
		}

	case pushMsg:
		m := msg.model
		// Update tab name when a named view is pushed.
		if titler, ok := m.(Titler); ok {
			t.tabs[t.active].name = titler.Title()
		}
		t.tabs[t.active].stack = append(t.tabs[t.active].stack, m)
		return t, m.Init()

	case popMsg:
		stack := t.tabs[t.active].stack
		if len(stack) <= 1 {
			// Popping the last item closes the tab.
			return t.closeActive()
		}
		t.tabs[t.active].stack = stack[:len(stack)-1]
		// Restore tab name from the new top via Titler; fall back to "menu" for
		// models that don't implement it.
		if titler, ok := t.tabs[t.active].top().(Titler); ok {
			t.tabs[t.active].name = titler.Title()
		} else {
			t.tabs[t.active].name = "menu"
		}
		return t, nil
	}

	// Forward all other messages to the active top model.
	active := &t.tabs[t.active]
	updated, cmd := active.top().Update(msg)
	active.stack[len(active.stack)-1] = updated
	return t, cmd
}

func (t tabsModel) View() string {
	tabInfos := make([]TabInfo, len(t.tabs))
	for i, tb := range t.tabs {
		tabInfos[i] = TabInfo{Name: tb.name}
	}

	top := t.tabs[t.active].top()

	toolName := "conch"
	if titler, ok := top.(Titler); ok {
		toolName = titler.Title()
	}

	helpLine := "ctrl+t new tab  ctrl+w close  ctrl+←/→ switch"
	if helper, ok := top.(Helper); ok {
		helpLine = helper.HelpLine()
	}

	body := top.View()
	return RenderChrome(tabInfos, t.active, toolName, body, helpLine, t.w, t.h)
}

// closeActive removes the active tab. If it was the last tab, quit; otherwise
// activate the previous tab (or tab 0 if active was 0).
func (t tabsModel) closeActive() (tea.Model, tea.Cmd) {
	if len(t.tabs) == 1 {
		return t, tea.Quit
	}
	t.tabs = append(t.tabs[:t.active], t.tabs[t.active+1:]...)
	if t.active >= len(t.tabs) {
		t.active = len(t.tabs) - 1
	}
	return t, nil
}
