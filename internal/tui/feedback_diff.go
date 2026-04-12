package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/git"
	tea "github.com/charmbracelet/bubbletea"
)

type diffState int

const (
	stateCommits diffState = iota
	stateFiles
)

type diffFocus int

const (
	focusLeft diffFocus = iota
	focusRight
)

// diffView: commits (A) → files + diff preview (B).
// Notes panel appears in B when the selected file has notes or editing is active.
type diffView struct {
	ticket db.Ticket
	state  diffState
	focus  diffFocus
	// state A
	commits       []git.Commit
	commitCur     int
	commitFiles   []string
	commitMessage string
	// state B
	fileCur    int
	fileScroll int
	hunks      []git.DiffHunk
	allNotes   map[string][]db.FeedbackNote
	// note editor — note attaches to first hunk of selected file
	editing    bool
	editText   string
	editNoteID int64
	noteCur    int
	// misc
	loaded bool
	status string
	w, h   int
}

type diffLoadedMsg struct {
	commits []git.Commit
	err     string
}
type diffFilesMsg struct {
	files   []string
	message string
}
type diffHunksMsg struct {
	hunks    []git.DiffHunk
	allNotes map[string][]db.FeedbackNote
	err      string
}

func newDiffView(ticket db.Ticket, w, h int) diffView {
	return diffView{ticket: ticket, allNotes: make(map[string][]db.FeedbackNote), w: w, h: h}
}

func (v diffView) Title() string { return "Diff — " + v.ticket.Title }
func (v diffView) HelpLine() string {
	if v.state == stateCommits {
		return "↑/↓ navigate  enter select  esc back"
	}
	if v.focus == focusRight {
		return "↑/↓/pgdn/space scroll  h focus files  n note  e edit  d delete  esc back"
	}
	return "↑/↓ navigate files  l focus diff  n note  esc back"
}

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

func hunkKey(h git.DiffHunk) string { return h.FilePath + "\x00" + h.HunkHeader }

func (v diffView) filesFromHunks() []string {
	seen := map[string]bool{}
	var out []string
	for _, h := range v.hunks {
		if !seen[h.FilePath] {
			seen[h.FilePath] = true
			out = append(out, h.FilePath)
		}
	}
	return out
}

func (v diffView) hunksForFile(fp string) []git.DiffHunk {
	var out []git.DiffHunk
	for _, h := range v.hunks {
		if h.FilePath == fp {
			out = append(out, h)
		}
	}
	return out
}

// selectedFile returns the currently highlighted file path.
func (v diffView) selectedFile() string {
	files := v.filesFromHunks()
	if len(files) == 0 || v.fileCur >= len(files) {
		return ""
	}
	return files[v.fileCur]
}

// notesForFile returns all notes across all hunks of the selected file.
func (v diffView) notesForFile() []db.FeedbackNote {
	var out []db.FeedbackNote
	for _, h := range v.hunksForFile(v.selectedFile()) {
		out = append(out, v.allNotes[hunkKey(h)]...)
	}
	return out
}

// firstHunkForFile returns the first hunk of the selected file (for note attachment).
func (v diffView) firstHunkForFile() (git.DiffHunk, bool) {
	hunks := v.hunksForFile(v.selectedFile())
	if len(hunks) == 0 {
		return git.DiffHunk{}, false
	}
	return hunks[0], true
}

func hunkContentLines(h git.DiffHunk) []string {
	out := make([]string, 0, 1+len(h.Lines))
	out = append(out, h.HunkHeader)
	return append(out, h.Lines...)
}

func (v diffView) filePreviewLines() []string {
	var out []string
	for _, h := range v.hunksForFile(v.selectedFile()) {
		out = append(out, hunkContentLines(h)...)
	}
	return out
}

// contentHeight is the usable scrollable rows: terminal height minus chrome (2) and panel title (1).
func (v diffView) contentHeight() int {
	h := v.h - 3
	if h < 1 {
		h = 1
	}
	return h
}

func loadFilesCmd(worktreePath, hash string) tea.Cmd {
	return func() tea.Msg {
		files, _ := git.FilesChanged(worktreePath, hash)
		msg, _ := git.CommitMessage(worktreePath, hash)
		return diffFilesMsg{files: files, message: msg}
	}
}

func loadHunksCmd(worktreePath, hash string, ticketID int64) tea.Cmd {
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
		if len(v.commits) > 0 {
			return v, loadFilesCmd(v.ticket.WorktreePath, v.commits[0].Hash)
		}
	case diffFilesMsg:
		v.commitFiles = msg.files
		v.commitMessage = msg.message
	case diffHunksMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
			return v, nil
		}
		if msg.hunks != nil {
			v.hunks = msg.hunks
			v.fileScroll = 0
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
		v.editing = false
		v.editText = ""
		v.editNoteID = 0
		return v, cmd
	default:
		if len(msg.Runes) > 0 {
			v.editText += string(msg.Runes)
		}
	}
	return v, nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (v diffView) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ch := v.contentHeight()
	switch msg.String() {
	case "esc":
		if v.state == stateFiles {
			v.state = stateCommits
			v.fileCur = 0
			v.fileScroll = 0
			v.focus = focusLeft
			return v, nil
		}
		return v, pop()

	case "h", "left":
		v.focus = focusLeft
	case "l", "right":
		if v.state == stateFiles {
			v.focus = focusRight
		}

	case "up", "k":
		if v.state == stateCommits {
			if v.commitCur > 0 {
				v.commitCur--
				return v, loadFilesCmd(v.ticket.WorktreePath, v.commits[v.commitCur].Hash)
			}
		} else if v.focus == focusLeft {
			if v.fileCur > 0 {
				v.fileCur--
				v.fileScroll = 0
			}
		} else {
			v.fileScroll = clamp(v.fileScroll-1, 0, len(v.filePreviewLines())-ch)
		}

	case "down", "j":
		if v.state == stateCommits {
			if v.commitCur < len(v.commits)-1 {
				v.commitCur++
				return v, loadFilesCmd(v.ticket.WorktreePath, v.commits[v.commitCur].Hash)
			}
		} else if v.focus == focusLeft {
			files := v.filesFromHunks()
			if v.fileCur < len(files)-1 {
				v.fileCur++
				v.fileScroll = 0
			}
		} else {
			v.fileScroll = clamp(v.fileScroll+1, 0, len(v.filePreviewLines())-ch)
		}

	case "pgup":
		if v.state == stateFiles && v.focus == focusRight {
			v.fileScroll = clamp(v.fileScroll-ch, 0, len(v.filePreviewLines())-ch)
		}

	case "pgdown", " ":
		if v.state == stateFiles && v.focus == focusRight {
			v.fileScroll = clamp(v.fileScroll+ch, 0, len(v.filePreviewLines())-ch)
		}

	case "enter":
		if v.state == stateCommits && len(v.commits) > 0 {
			c := v.commits[v.commitCur]
			v.state = stateFiles
			v.fileCur = 0
			v.fileScroll = 0
			v.focus = focusLeft
			return v, loadHunksCmd(v.ticket.WorktreePath, c.Hash, v.ticket.ID)
		}

	case "n":
		if v.state == stateFiles {
			v.editing = true
			v.editNoteID = 0
			v.editText = ""
		}

	case "e":
		if v.state == stateFiles {
			notes := v.notesForFile()
			if v.noteCur < len(notes) {
				n := notes[v.noteCur]
				v.editing = true
				v.editNoteID = n.ID
				v.editText = n.Body
			}
		}

	case "d":
		if v.state == stateFiles {
			notes := v.notesForFile()
			if v.noteCur < len(notes) {
				noteID := notes[v.noteCur].ID
				ticketID := v.ticket.ID
				return v, func() tea.Msg {
					client.Send(client.Request{Action: "delete_feedback_note", NoteID: noteID}) //nolint:errcheck
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

func (v diffView) saveNoteCmd() tea.Cmd {
	noteID := v.editNoteID
	body := v.editText
	ticketID := v.ticket.ID
	var commitHash, filePath, hunkHeader string
	if len(v.commits) > 0 {
		commitHash = v.commits[v.commitCur].Hash
	}
	if h, ok := v.firstHunkForFile(); ok {
		filePath = h.FilePath
		hunkHeader = h.HunkHeader
	}
	return func() tea.Msg {
		if noteID == 0 {
			client.Send(client.Request{ //nolint:errcheck
				Action: "create_feedback_note", TicketID: ticketID,
				CommitHash: commitHash, FilePath: filePath,
				HunkHeader: hunkHeader, NoteBody: body,
			})
		} else {
			client.Send(client.Request{Action: "update_feedback_note", NoteID: noteID, NoteBody: body}) //nolint:errcheck
		}
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

// --- View ---

func (v diffView) View() string {
	if !v.loaded {
		return "  loading...\n"
	}
	if v.state == stateCommits {
		return v.viewCommits()
	}
	return v.viewFiles()
}

func (v diffView) viewCommits() string {
	leftW, rightW := v.w/3, v.w-v.w/3
	if leftW < 1 {
		leftW = 1
	}
	if rightW < 1 {
		rightW = 1
	}
	var left strings.Builder
	left.WriteString(StyleTitle.Width(leftW).Render("Commits") + "\n")
	for i, c := range v.commits {
		subj := truncate(c.Subject, leftW-2)
		if i == v.commitCur {
			left.WriteString(StyleCursor.Width(leftW).Render("> "+subj) + "\n")
		} else {
			left.WriteString(StyleBody.Width(leftW).Render("  "+subj) + "\n")
		}
	}
	if len(v.commits) == 0 {
		left.WriteString(StyleBody.Width(leftW).Render("  no commits") + "\n")
	}
	var right strings.Builder
	right.WriteString(StyleTitle.Width(rightW).Render("Files Changed") + "\n")
	if v.commitMessage != "" {
		for _, line := range strings.Split(strings.TrimSpace(v.commitMessage), "\n") {
			for _, wrapped := range wordWrap(line, rightW-2) {
				right.WriteString(StyleBody.Width(rightW).Render("  "+wrapped) + "\n")
			}
		}
		right.WriteString(StyleBody.Width(rightW).Render("") + "\n")
	}
	for _, f := range v.commitFiles {
		right.WriteString(StyleBody.Width(rightW).Render("  "+truncate(f, rightW-2)) + "\n")
	}
	if len(v.commitFiles) == 0 {
		right.WriteString(StyleBody.Width(rightW).Render("  —") + "\n")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left.String(), right.String())
}

func (v diffView) viewFiles() string {
	showNotes := len(v.notesForFile()) > 0 || v.editing
	leftW := v.w / 4
	var midW, notesW int
	if showNotes {
		notesW = v.w / 4
		midW = v.w - leftW - notesW
	} else {
		midW = v.w - leftW
	}
	if leftW < 1 {
		leftW = 1
	}
	if midW < 1 {
		midW = 1
	}

	files := v.filesFromHunks()
	var left strings.Builder
	left.WriteString(StyleTitle.Width(leftW).Render(panelTitle("Files", v.focus == focusLeft)) + "\n")
	for i, f := range files {
		if i == v.fileCur {
			left.WriteString(StyleCursor.Width(leftW).Render("> "+truncate(f, leftW-2)) + "\n")
		} else {
			// show note indicator
			marker := "  "
			for _, h := range v.hunksForFile(f) {
				if len(v.allNotes[hunkKey(h)]) > 0 {
					marker = "● "
					break
				}
			}
			left.WriteString(StyleBody.Width(leftW).Render(marker+truncate(f, leftW-2)) + "\n")
		}
	}
	if len(files) == 0 {
		left.WriteString(StyleBody.Width(leftW).Render("  no files") + "\n")
	}

	ch := v.contentHeight()
	var mid strings.Builder
	mid.WriteString(StyleTitle.Width(midW).Render(panelTitle("Diff", v.focus == focusRight)) + "\n")
	lines := v.filePreviewLines()
	for _, line := range scrollWindow(lines, v.fileScroll, ch) {
		mid.WriteString(renderDiffLine(line, midW))
	}
	if len(lines) == 0 {
		mid.WriteString(StyleBody.Width(midW).Render("  —") + "\n")
	}
	if len(lines) > ch {
		mid.WriteString(StyleBody.Width(midW).Render(scrollPct(v.fileScroll, ch, len(lines))) + "\n")
	}

	if !showNotes {
		return lipgloss.JoinHorizontal(lipgloss.Top, left.String(), mid.String())
	}

	var notes strings.Builder
	notes.WriteString(StyleTitle.Width(notesW).Render("Notes") + "\n")
	if v.editing {
		notes.WriteString(StyleBorder.Width(notesW-2).Render(v.editText+"█") + "\n")
		notes.WriteString(StyleBody.Width(notesW).Render("  enter save  esc cancel") + "\n")
	} else {
		for i, n := range v.notesForFile() {
			body := truncate(n.Body, notesW-4)
			if i == v.noteCur {
				notes.WriteString(StyleCursor.Width(notesW).Render("> "+body) + "\n")
			} else {
				notes.WriteString(StyleBody.Width(notesW).Render("  "+body) + "\n")
			}
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left.String(), mid.String(), notes.String())
}

var (
	styleAddLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#50fa7b"))
	styleDelLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555"))
)

func panelTitle(title string, active bool) string {
	if active {
		return StyleCursor.Render(title)
	}
	return title
}

func renderDiffLine(line string, w int) string {
	rendered := truncate(line, w-2)
	switch {
	case strings.HasPrefix(line, "+"):
		return styleAddLine.Width(w).Render("  "+rendered) + "\n"
	case strings.HasPrefix(line, "-"):
		return styleDelLine.Width(w).Render("  "+rendered) + "\n"
	default:
		return StyleBody.Width(w).Render("  "+rendered) + "\n"
	}
}

func scrollWindow(lines []string, offset, height int) []string {
	if offset >= len(lines) || height < 1 {
		return nil
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end]
}

func scrollPct(offset, height, total int) string {
	if total == 0 {
		return "  -- 100% --"
	}
	pct := 100 * (offset + height) / total
	if pct > 100 {
		pct = 100
	}
	return "  -- " + itoa(pct) + "% --"
}

func truncate(s string, max int) string {
	if max < 1 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}

func wordWrap(s string, width int) []string {
	if width < 1 || s == "" {
		return []string{s}
	}
	var lines []string
	for _, word := range strings.Fields(s) {
		if len(lines) == 0 {
			lines = append(lines, word)
			continue
		}
		last := lines[len(lines)-1]
		if len([]rune(last))+1+len([]rune(word)) <= width {
			lines[len(lines)-1] = last + " " + word
		} else {
			lines = append(lines, word)
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
