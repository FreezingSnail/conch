package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
	tea "github.com/charmbracelet/bubbletea"
)

// diffFocus identifies which panel has keyboard focus.
type diffFocus int

const (
	focusLeft   diffFocus = iota // commit list
	focusCenter                  // diff hunk list
	focusRight                   // notes for the selected hunk
)

// diffView is a three-panel viewer: commits (left), diff hunks (center),
// and feedback notes for the selected hunk (right). It is pushed from
// feedbackView when the user presses enter on a ticket.
//
// When editing=true the right panel shows an inline textarea. editNoteID==0
// means a new note is being created; non-zero means an existing note is being
// updated. noteCur is the cursor within the notes list for the current hunk.
type diffView struct {
	ticket     db.Ticket
	commits    []git.Commit
	commitCur  int
	hunks      []git.DiffHunk
	hunkCur    int
	allNotes   map[string][]db.FeedbackNote // keyed by hunkKey
	focus      diffFocus
	editing    bool
	editText   string
	editNoteID int64 // 0 = new note; non-zero = update existing
	noteCur    int   // cursor within the notes list for the current hunk
	loaded     bool
	status     string
	w, h       int
}

// diffLoadedMsg carries the initial commit list after Init fires.
type diffLoadedMsg struct {
	commits []git.Commit
	err     string
}

// diffHunksMsg carries the parsed hunks and notes after a commit is selected.
type diffHunksMsg struct {
	hunks    []git.DiffHunk
	allNotes map[string][]db.FeedbackNote
	err      string
}

// newDiffView constructs a diffView for the given ticket.
func newDiffView(ticket db.Ticket) diffView {
	return diffView{ticket: ticket, allNotes: make(map[string][]db.FeedbackNote)}
}

// Title implements Titler; used by the tab bar chrome.
func (v diffView) Title() string { return "Diff — " + v.ticket.Title }

// HelpLine implements Helper; returns context-sensitive keybinding hints.
func (v diffView) HelpLine() string {
	return "h/l focus panel  j/k navigate  enter load commit  n add note  esc back"
}

// Init kicks off the commit log load for the ticket's worktree.
func (v diffView) Init() tea.Cmd {
	path := v.ticket.WorktreePath
	return func() tea.Msg {
		commits, err := git.LogList(path)
		if err != nil {
			return diffLoadedMsg{err: err.Error()}
		}
		return diffLoadedMsg{commits: commits}
	}
}

// hunkKey returns the map key used to associate notes with a hunk.
// The null byte separator prevents collisions between file path and header.
func hunkKey(h git.DiffHunk) string { return h.FilePath + "\x00" + h.HunkHeader }

// loadHunksCmd fetches the diff for the given commit and all notes for the ticket,
// then indexes notes by hunk key.
func loadHunksCmd(worktreePath string, hash string, ticketID int64) tea.Cmd {
	return func() tea.Msg {
		raw, err := git.DiffCommit(worktreePath, hash)
		if err != nil {
			return diffHunksMsg{err: err.Error()}
		}
		hunks := git.ParseDiff(raw)

		nr, err := client.Send(client.Request{Action: "list_feedback_notes", TicketID: ticketID})
		allNotes := make(map[string][]db.FeedbackNote)
		if err == nil && nr.OK {
			for _, n := range nr.FeedbackNotes {
				k := n.FilePath + "\x00" + n.HunkHeader
				allNotes[k] = append(allNotes[k], n)
			}
		}
		return diffHunksMsg{hunks: hunks, allNotes: allNotes}
	}
}

func (v diffView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.w, v.h = msg.Width, msg.Height

	case diffLoadedMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
			v.loaded = true
			return v, nil
		}
		v.commits = msg.commits
		v.loaded = true
		// Auto-load the first commit's diff if one exists.
		if len(v.commits) > 0 {
			return v, loadHunksCmd(v.ticket.WorktreePath, v.commits[0].Hash, v.ticket.ID)
		}

	case diffHunksMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
			return v, nil
		}
		// A nil hunks slice signals a notes-only reload (after a mutation);
		// preserve the existing hunk list in that case.
		if msg.hunks != nil {
			v.hunks = msg.hunks
			v.hunkCur = 0
		}
		v.allNotes = msg.allNotes
		v.status = ""

	case tea.KeyMsg:
		if v.editing {
			return v.updateEditing(msg)
		}
		return v.updateNormal(msg)
	}
	return v, nil
}

// updateEditing handles key events while the inline note editor is open.
// Printable runes accumulate into editText; enter saves; esc cancels.
func (v diffView) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.editing = false
		v.editText = ""
		v.editNoteID = 0
	case "backspace":
		if len(v.editText) > 0 {
			v.editText = v.editText[:len(v.editText)-1]
		}
	case "enter":
		cmd := v.saveNoteCmd()
		// Clear editor state on the returned model; saveNoteCmd captures the
		// pre-clear values it needs via closure before this point.
		v.editing = false
		v.editText = ""
		v.editNoteID = 0
		return v, cmd
	default:
		// Append printable runes only.
		if len(msg.Runes) > 0 {
			v.editText += string(msg.Runes)
		}
	}
	return v, nil
}

// saveNoteCmd builds the IPC command to create or update a note, then reloads
// all notes for the current commit. editNoteID==0 triggers create; non-zero triggers update.
// The caller is responsible for clearing editing state on the returned model.
func (v diffView) saveNoteCmd() tea.Cmd {
	// Capture values needed inside the closure.
	noteID := v.editNoteID
	body := v.editText
	ticketID := v.ticket.ID

	var commitHash, filePath, hunkHeader string
	if len(v.commits) > 0 {
		commitHash = v.commits[v.commitCur].Hash
	}
	if len(v.hunks) > 0 && v.hunkCur < len(v.hunks) {
		filePath = v.hunks[v.hunkCur].FilePath
		hunkHeader = v.hunks[v.hunkCur].HunkHeader
	}

	return func() tea.Msg {
		if noteID == 0 {
			client.Send(client.Request{ //nolint:errcheck
				Action:     "create_feedback_note",
				TicketID:   ticketID,
				CommitHash: commitHash,
				FilePath:   filePath,
				HunkHeader: hunkHeader,
				NoteBody:   body,
			})
		} else {
			client.Send(client.Request{ //nolint:errcheck
				Action:   "update_feedback_note",
				NoteID:   noteID,
				NoteBody: body,
			})
		}
		// Reload notes after mutation.
		nr, err := client.Send(client.Request{Action: "list_feedback_notes", TicketID: ticketID})
		allNotes := make(map[string][]db.FeedbackNote)
		if err == nil && nr.OK {
			for _, n := range nr.FeedbackNotes {
				k := n.FilePath + "\x00" + n.HunkHeader
				allNotes[k] = append(allNotes[k], n)
			}
		}
		return diffHunksMsg{hunks: nil, allNotes: allNotes}
	}
}

// updateNormal handles key events when the editor is closed.
func (v diffView) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return v, pop()
	case "h", "left":
		if v.focus > focusLeft {
			v.focus--
		}
	case "l", "right":
		if v.focus < focusRight {
			v.focus++
		}
	case "j", "down":
		v = v.cursorDown()
	case "k", "up":
		v = v.cursorUp()
	case "enter":
		if v.focus == focusLeft && len(v.commits) > 0 {
			c := v.commits[v.commitCur]
			return v, loadHunksCmd(v.ticket.WorktreePath, c.Hash, v.ticket.ID)
		}
	case "n":
		// Open editor for a new note on the current hunk (available from any focus).
		v.editing = true
		v.editNoteID = 0
		v.editText = ""
	case "e":
		// Edit the note under noteCur in the right panel.
		if v.focus == focusRight {
			notes := v.currentNotes()
			if v.noteCur < len(notes) {
				n := notes[v.noteCur]
				v.editing = true
				v.editNoteID = n.ID
				v.editText = n.Body
			}
		}
	case "d":
		// Delete the note under noteCur in the right panel.
		if v.focus == focusRight {
			notes := v.currentNotes()
			if v.noteCur < len(notes) {
				noteID := notes[v.noteCur].ID
				ticketID := v.ticket.ID
				return v, func() tea.Msg {
					client.Send(client.Request{ //nolint:errcheck
						Action: "delete_feedback_note",
						NoteID: noteID,
					})
					nr, err := client.Send(client.Request{Action: "list_feedback_notes", TicketID: ticketID})
					allNotes := make(map[string][]db.FeedbackNote)
					if err == nil && nr.OK {
						for _, n := range nr.FeedbackNotes {
							k := n.FilePath + "\x00" + n.HunkHeader
							allNotes[k] = append(allNotes[k], n)
						}
					}
					return diffHunksMsg{allNotes: allNotes}
				}
			}
		}
	}
	return v, nil
}

// currentNotes returns the notes slice for the currently selected hunk.
func (v diffView) currentNotes() []db.FeedbackNote {
	if len(v.hunks) == 0 || v.hunkCur >= len(v.hunks) {
		return nil
	}
	return v.allNotes[hunkKey(v.hunks[v.hunkCur])]
}

// cursorDown advances the cursor in the focused panel.
func (v diffView) cursorDown() diffView {
	switch v.focus {
	case focusLeft:
		if v.commitCur < len(v.commits)-1 {
			v.commitCur++
		}
	case focusCenter:
		if v.hunkCur < len(v.hunks)-1 {
			v.hunkCur++
		}
	case focusRight:
		notes := v.currentNotes()
		if v.noteCur < len(notes)-1 {
			v.noteCur++
		}
	}
	return v
}

// cursorUp retreats the cursor in the focused panel.
func (v diffView) cursorUp() diffView {
	switch v.focus {
	case focusLeft:
		if v.commitCur > 0 {
			v.commitCur--
		}
	case focusCenter:
		if v.hunkCur > 0 {
			v.hunkCur--
		}
	case focusRight:
		if v.noteCur > 0 {
			v.noteCur--
		}
	}
	return v
}

// View renders the three-panel layout. Panel widths are derived from the
// terminal width: left = w/4, center = w/2, right = remainder.
func (v diffView) View() string {
	if !v.loaded {
		return "  loading...\n"
	}

	leftW := v.w / 4
	centerW := v.w / 2
	rightW := v.w - leftW - centerW
	if leftW < 1 {
		leftW = 1
	}
	if centerW < 1 {
		centerW = 1
	}
	if rightW < 1 {
		rightW = 1
	}

	left := v.renderLeft(leftW)
	center := v.renderCenter(centerW)
	right := v.renderRight(rightW)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

// renderLeft renders the commit list panel, constrained to w columns.
func (v diffView) renderLeft(w int) string {
	var b strings.Builder
	b.WriteString(StyleTitle.Width(w).Render("Commits") + "\n")
	for i, c := range v.commits {
		subj := c.Subject
		// Truncate subject to fit within the panel minus the "  " prefix.
		maxLen := w - 2
		if maxLen < 1 {
			maxLen = 1
		}
		if len(subj) > maxLen {
			subj = subj[:maxLen]
		}
		line := "  " + subj
		if i == v.commitCur && v.focus == focusLeft {
			b.WriteString(StyleCursor.Width(w).Render("> "+subj) + "\n")
		} else if i == v.commitCur {
			b.WriteString(StyleCursor.Width(w).Render(line) + "\n")
		} else {
			b.WriteString(StyleBody.Width(w).Render(line) + "\n")
		}
	}
	if len(v.commits) == 0 {
		b.WriteString(StyleBody.Width(w).Render("  no commits") + "\n")
	}
	return b.String()
}

// renderCenter renders the diff hunk list panel. Hunks that have at least one
// note are prefixed with ● to signal existing feedback.
func (v diffView) renderCenter(w int) string {
	var b strings.Builder
	b.WriteString(StyleTitle.Width(w).Render("Hunks") + "\n")
	for i, h := range v.hunks {
		marker := "  "
		if len(v.allNotes[hunkKey(h)]) > 0 {
			marker = "● "
		}
		label := h.FilePath + " " + h.HunkHeader
		maxLen := w - len(marker)
		if maxLen < 1 {
			maxLen = 1
		}
		if len(label) > maxLen {
			label = label[:maxLen]
		}
		line := marker + label
		if i == v.hunkCur {
			b.WriteString(StyleCursor.Width(w).Render(line) + "\n")
		} else {
			b.WriteString(StyleBody.Width(w).Render(line) + "\n")
		}
	}
	if len(v.hunks) == 0 {
		b.WriteString(StyleBody.Width(w).Render("  no hunks") + "\n")
	}
	return b.String()
}

// renderRight renders the notes panel for the currently selected hunk.
// When editing=true it shows an inline bordered textarea instead of the note list.
func (v diffView) renderRight(w int) string {
	var b strings.Builder
	b.WriteString(StyleTitle.Width(w).Render("Notes") + "\n")

	if v.editing {
		// Inline editor: bordered text area with the current editText and a hint line.
		inner := v.editText + "█" // block cursor
		b.WriteString(StyleBorder.Width(w-2).Render(inner) + "\n")
		b.WriteString(StyleBody.Width(w).Render("  [enter save  esc cancel]") + "\n")
		return b.String()
	}

	notes := v.currentNotes()
	for i, n := range notes {
		body := n.Body
		if len(body) > w-4 {
			body = body[:w-4]
		}
		line := "  " + body
		if i == v.noteCur && v.focus == focusRight {
			b.WriteString(StyleCursor.Width(w).Render("> "+body) + "\n")
		} else {
			b.WriteString(StyleBody.Width(w).Render(line) + "\n")
		}
	}
	if len(notes) == 0 {
		b.WriteString(StyleBody.Width(w).Render("  no notes") + "\n")
	}
	return b.String()
}
