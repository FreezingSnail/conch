package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- mantleView ---

// TestMantleView_implementsTitler verifies Title() returns "Mantle".
func TestMantleView_implementsTitler(t *testing.T) {
	v := newMantleView()
	if v.Title() != "Mantle" {
		t.Errorf("want %q, got %q", "Mantle", v.Title())
	}
}

// TestMantleView_implementsHelper verifies HelpLine() is non-empty in all
// relevant states.
func TestMantleView_implementsHelper(t *testing.T) {
	v := newMantleView()
	if v.HelpLine() == "" {
		t.Error("HelpLine() must not be empty in nav state")
	}
	v.content = "some content"
	if v.HelpLine() == "" {
		t.Error("HelpLine() must not be empty in reader state")
	}
}

// TestMantleView_cursorRowStyled verifies the cursor row uses the ">" prefix.
func TestMantleView_cursorRowStyled(t *testing.T) {
	v := newMantleView()
	v.agents = []string{"my-agent"}
	v.cursor = 0
	out := v.View()
	if !strings.Contains(out, "> my-agent") {
		t.Errorf("expected cursor row '> my-agent' in output:\n%s", out)
	}
}

// TestMantleView_noInlineHelp verifies View() does not embed help text.
func TestMantleView_noInlineHelp(t *testing.T) {
	v := newMantleView()
	out := v.View()
	if strings.Contains(out, "esc back") {
		t.Errorf("View() must not contain inline help text; found 'esc back' in:\n%s", out)
	}
}

// --- planningWizard ---

// TestPlanningWizard_implementsTitler verifies Title() returns "Plan".
func TestPlanningWizard_implementsTitler(t *testing.T) {
	w := newPlanningWizard()
	if w.Title() != "Plan" {
		t.Errorf("want %q, got %q", "Plan", w.Title())
	}
}

// TestPlanningWizard_implementsHelper verifies HelpLine() is non-empty in all
// wizard steps.
func TestPlanningWizard_implementsHelper(t *testing.T) {
	w := newPlanningWizard()
	w.step = stepTicketNum
	if w.HelpLine() == "" {
		t.Error("HelpLine() must not be empty at stepTicketNum")
	}
	w.step = stepRepoPicker
	if w.HelpLine() == "" {
		t.Error("HelpLine() must not be empty at stepRepoPicker")
	}
	w.step = stepSummary
	if w.HelpLine() == "" {
		t.Error("HelpLine() must not be empty at stepSummary")
	}
}

// TestPlanningWizard_cursorRowStyled verifies the cursor row uses ">" in the
// repo picker step.
func TestPlanningWizard_cursorRowStyled(t *testing.T) {
	w := newPlanningWizard()
	w.step = stepRepoPicker
	w.repos = []string{"/work/alpha", "/work/beta"}
	w.repoSel = []bool{false, false}
	w.loaded = true
	w.repoCur = 0
	out := w.View()
	if !strings.Contains(out, "> [ ] alpha") {
		t.Errorf("expected cursor row '> [ ] alpha' in output:\n%s", out)
	}
}

// TestPlanningWizard_errorStatusStyled verifies an error status appears in
// the repo picker view.
func TestPlanningWizard_errorStatusStyled(t *testing.T) {
	w := newPlanningWizard()
	w.step = stepRepoPicker
	w.repos = []string{"/work/alpha"}
	w.repoSel = []bool{false}
	w.loaded = true
	w.status = "error: select at least one repo"
	out := w.View()
	if !strings.Contains(out, "error: select at least one repo") {
		t.Errorf("expected error status in output:\n%s", out)
	}
}

// TestPlanningWizard_noInlineHelp verifies View() does not embed help text.
func TestPlanningWizard_noInlineHelp(t *testing.T) {
	w := newPlanningWizard()
	w.step = stepRepoPicker
	w.repos = []string{"/work/alpha"}
	w.repoSel = []bool{false}
	w.loaded = true
	out := w.View()
	if strings.Contains(out, "esc back") {
		t.Errorf("View() must not contain inline help text; found 'esc back' in:\n%s", out)
	}
}

// --- planningSessionsView ---

// TestPlanningSessionsView_implementsTitler verifies Title() returns
// "Planning Sessions".
func TestPlanningSessionsView_implementsTitler(t *testing.T) {
	v := newPlanningSessionsView()
	if v.Title() != "Planning Sessions" {
		t.Errorf("want %q, got %q", "Planning Sessions", v.Title())
	}
}

// TestPlanningSessionsView_implementsHelper verifies HelpLine() is non-empty.
func TestPlanningSessionsView_implementsHelper(t *testing.T) {
	v := newPlanningSessionsView()
	if v.HelpLine() == "" {
		t.Error("HelpLine() must not be empty")
	}
}

// TestPlanningSessionsView_noInlineHelp verifies View() does not embed help
// text.
func TestPlanningSessionsView_noInlineHelp(t *testing.T) {
	v := newPlanningSessionsView()
	v.loaded = true
	out := v.View()
	if strings.Contains(out, "esc back") {
		t.Errorf("View() must not contain inline help text; found 'esc back' in:\n%s", out)
	}
}

// TestPlanningSessionsView_errorStatusStyled verifies an error status appears
// in View() output.
func TestPlanningSessionsView_errorStatusStyled(t *testing.T) {
	v := newPlanningSessionsView()
	v.loaded = true
	v.status = "error: no worktree"
	out := v.View()
	if !strings.Contains(out, "error: no worktree") {
		t.Errorf("expected error status in output:\n%s", out)
	}
}

// TestPlanningSessionsView_windowSizeMsg verifies w and h are updated from
// tea.WindowSizeMsg.
func TestPlanningSessionsView_windowSizeMsg(t *testing.T) {
	v := newPlanningSessionsView()
	updated, _ := v.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	pv := updated.(planningSessionsView)
	if pv.w != 120 || pv.h != 40 {
		t.Errorf("want w=120 h=40, got w=%d h=%d", pv.w, pv.h)
	}
}
