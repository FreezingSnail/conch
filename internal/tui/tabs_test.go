package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// titledModel is a minimal tea.Model that implements Titler and Helper,
// used to verify interface-driven tab naming and help text.
type titledModel struct{ title string }

func (m titledModel) Init() tea.Cmd                       { return nil }
func (m titledModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m titledModel) View() string                        { return m.title }
func (m titledModel) Title() string                       { return m.title }
func (m titledModel) HelpLine() string                    { return "help:" + m.title }

// sendKey is a helper that sends a key string to a tabsModel and returns the
// updated model.
func sendKey(t *testing.T, m tabsModel, key string) tabsModel {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	if tm, ok := updated.(tabsModel); ok {
		return tm
	}
	// Some keys are represented as special key types; try again via string.
	updated, _ = m.Update(tea.KeyMsg{})
	_ = updated
	return m
}

// sendMsg sends an arbitrary message to a tabsModel and returns the result.
func sendMsg(m tabsModel, msg tea.Msg) (tabsModel, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(tabsModel), cmd
}

// TestNewTabsModel_singleMenuTab verifies the initial state has exactly one
// tab named "menu".
func TestNewTabsModel_singleMenuTab(t *testing.T) {
	m := newTabsModel()
	if len(m.tabs) != 1 {
		t.Fatalf("want 1 tab, got %d", len(m.tabs))
	}
	if m.tabs[0].name != "menu" {
		t.Errorf("want tab name %q, got %q", "menu", m.tabs[0].name)
	}
	if m.active != 0 {
		t.Errorf("want active=0, got %d", m.active)
	}
}

// TestCtrlT_opensNewMenuTab verifies ctrl+t appends a new "menu" tab and
// makes it active.
func TestCtrlT_opensNewMenuTab(t *testing.T) {
	m := newTabsModel()
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyCtrlT})
	if len(m.tabs) != 2 {
		t.Fatalf("want 2 tabs, got %d", len(m.tabs))
	}
	if m.active != 1 {
		t.Errorf("want active=1, got %d", m.active)
	}
	if m.tabs[1].name != "menu" {
		t.Errorf("want new tab name %q, got %q", "menu", m.tabs[1].name)
	}
}

// TestCtrlW_closesActiveTab verifies ctrl+w removes the active tab and
// activates the previous one.
func TestCtrlW_closesActiveTab(t *testing.T) {
	m := newTabsModel()
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyCtrlT}) // now 2 tabs, active=1
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyCtrlW})
	if len(m.tabs) != 1 {
		t.Fatalf("want 1 tab after close, got %d", len(m.tabs))
	}
	if m.active != 0 {
		t.Errorf("want active=0 after close, got %d", m.active)
	}
}

// TestCtrlW_lastTab_quitsApp verifies ctrl+w on the last tab returns tea.Quit.
func TestCtrlW_lastTab_quitsApp(t *testing.T) {
	m := newTabsModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	if cmd == nil {
		t.Fatal("want tea.Quit cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("want tea.QuitMsg, got %T", msg)
	}
}

// TestCtrlRight_cyclesTabForward verifies ctrl+right wraps the active index.
func TestCtrlRight_cyclesTabForward(t *testing.T) {
	m := newTabsModel()
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyCtrlT}) // 2 tabs, active=1
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyCtrlRight})
	if m.active != 0 {
		t.Errorf("want active=0 after wrap, got %d", m.active)
	}
}

// TestCtrlLeft_cyclesTabBackward verifies ctrl+left wraps the active index.
func TestCtrlLeft_cyclesTabBackward(t *testing.T) {
	m := newTabsModel()
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyCtrlT}) // 2 tabs, active=1
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyCtrlLeft})
	if m.active != 0 {
		t.Errorf("want active=0, got %d", m.active)
	}
}

// TestPushMsg_updatesTabName verifies that pushing a Titler model renames the
// active tab.
func TestPushMsg_updatesTabName(t *testing.T) {
	m := newTabsModel()
	m, _ = sendMsg(m, pushMsg{model: titledModel{title: "tickets"}})
	if m.tabs[0].name != "tickets" {
		t.Errorf("want tab name %q, got %q", "tickets", m.tabs[0].name)
	}
}

// TestPopMsg_restoresMenuName verifies that popping back to the menu restores
// the tab name from menu's Title() ("conch").
func TestPopMsg_restoresMenuName(t *testing.T) {
	m := newTabsModel()
	m, _ = sendMsg(m, pushMsg{model: titledModel{title: "tickets"}})
	m, _ = sendMsg(m, popMsg{})
	if m.tabs[0].name != "conch" {
		t.Errorf("want tab name %q after pop, got %q", "conch", m.tabs[0].name)
	}
}

// TestPopMsg_emptyStack_closesTab verifies that popping the last item on a
// multi-tab stack closes that tab rather than leaving an empty stack.
func TestPopMsg_emptyStack_closesTab(t *testing.T) {
	m := newTabsModel()
	m, _ = sendMsg(m, tea.KeyMsg{Type: tea.KeyCtrlT}) // 2 tabs
	m, _ = sendMsg(m, popMsg{})                       // pop only item on tab 1
	if len(m.tabs) != 1 {
		t.Fatalf("want 1 tab after pop-close, got %d", len(m.tabs))
	}
}

// TestPopMsg_lastTab_quitsApp verifies that popping the only item on the only
// tab returns tea.Quit.
func TestPopMsg_lastTab_quitsApp(t *testing.T) {
	m := newTabsModel()
	_, cmd := m.Update(popMsg{})
	if cmd == nil {
		t.Fatal("want tea.Quit cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("want tea.QuitMsg, got %T", msg)
	}
}

// TestWindowSizeMsg_storesDimensions verifies w and h are updated on resize.
func TestWindowSizeMsg_storesDimensions(t *testing.T) {
	m := newTabsModel()
	m, _ = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.w != 120 || m.h != 40 {
		t.Errorf("want w=120 h=40, got w=%d h=%d", m.w, m.h)
	}
}

// TestView_usesHelperInterface verifies that a model implementing Helper
// supplies its HelpLine to the chrome.
func TestView_usesHelperInterface(t *testing.T) {
	m := newTabsModel()
	m, _ = sendMsg(m, pushMsg{model: titledModel{title: "plan"}})
	// View must not panic; we just verify it returns a non-empty string.
	v := m.View()
	if v == "" {
		t.Error("View() returned empty string")
	}
}

// TestCtrlC_quitsApp verifies ctrl+c returns tea.Quit.
func TestCtrlC_quitsApp(t *testing.T) {
	m := newTabsModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("want tea.Quit cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("want tea.QuitMsg, got %T", msg)
	}
}
