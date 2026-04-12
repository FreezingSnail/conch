package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	tea "github.com/charmbracelet/bubbletea"
)

type reviewTab int

const (
	reviewTabOpen      reviewTab = iota // PRs with status "open"
	reviewTabInReview                   // PRs with status "in_review"
	reviewTabReady                      // PRs with status "ready", shows comment table on enter
	reviewTabCompleted                  // PRs with status "completed", read-only
)

var reviewTabNames = []string{"Open PRs", "In Review", "Ready for Approval", "Completed"}

type reviewView struct {
	tab             reviewTab
	prs             []db.PullRequest
	comments        []db.PRReviewComment // comments for the selected ready PR
	cursor          int
	commentCursor   int
	viewingComments bool
	logLines        []string
	logScroll       int
	polling         bool
	loaded          bool
	status          string
	w, h            int
}

// reviewLoadedMsg carries the full PR list after a load.
type reviewLoadedMsg struct {
	prs []db.PullRequest
	err string
}

// reviewStartedMsg signals that a review_start action completed.
type reviewStartedMsg struct {
	err string
}

// reviewCommentsMsg carries comments for a selected PR.
type reviewCommentsMsg struct {
	comments []db.PRReviewComment
	err      string
}

// reviewLogPollMsg is the tick message for the In Review polling loop.
type reviewLogPollMsg struct{}

func newReviewView() reviewView { return reviewView{} }

// Title implements Titler.
func (v reviewView) Title() string { return "Review" }

// HelpLine returns context-sensitive keybinding hints.
func (v reviewView) HelpLine() string {
	switch {
	case v.tab == reviewTabReady && v.viewingComments:
		return "↑/↓ navigate  a toggle approve  p push approved  esc back"
	case v.tab == reviewTabOpen:
		return "tab switch tabs  ↑/↓ navigate  enter start review  r refresh  esc back"
	case v.tab == reviewTabReady:
		return "tab switch tabs  ↑/↓ navigate  enter view comments  r refresh  esc back"
	default:
		return "tab switch tabs  ↑/↓ navigate  r refresh  esc back"
	}
}

func (v reviewView) Init() tea.Cmd { return loadReviewView }

// loadReviewView fetches PRs for all four statuses and merges them.
func loadReviewView() tea.Msg {
	var all []db.PullRequest
	for _, status := range []string{"open", "in_review", "ready", "completed"} {
		resp, err := client.Send(client.Request{Action: "list_prs", Status: status})
		if err != nil {
			return reviewLoadedMsg{err: err.Error()}
		}
		if resp.OK {
			all = append(all, resp.PRs...)
		}
	}
	return reviewLoadedMsg{prs: all}
}

// tabPRs filters v.prs to those matching the active tab's status.
func (v reviewView) tabPRs() []db.PullRequest {
	statusMap := map[reviewTab]string{
		reviewTabOpen:      "open",
		reviewTabInReview:  "in_review",
		reviewTabReady:     "ready",
		reviewTabCompleted: "completed",
	}
	want := statusMap[v.tab]
	var out []db.PullRequest
	for _, pr := range v.prs {
		if pr.Status == want {
			out = append(out, pr)
		}
	}
	return out
}

// reviewPollCmd schedules the next In Review poll tick.
func reviewPollCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return reviewLogPollMsg{} })
}

func (v reviewView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.w, v.h = msg.Width, msg.Height

	case reviewLoadedMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
		}
		v.prs = msg.prs
		v.loaded = true
		v.polling = false
		// Start polling if the selected In Review PR has a running session.
		if v.tab == reviewTabInReview && len(v.tabPRs()) > 0 {
			v.polling = true
			return v, reviewPollCmd()
		}

	case reviewStartedMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
		} else {
			v.status = "review started — r to refresh"
			v.loaded = false
			return v, loadReviewView
		}

	case reviewCommentsMsg:
		if msg.err != "" {
			v.status = "error: " + msg.err
		} else {
			v.comments = msg.comments
			v.viewingComments = true
			v.commentCursor = 0
		}

	case reviewLogPollMsg:
		if v.tab != reviewTabInReview {
			v.polling = false
			return v, nil
		}
		// Reload PR list to detect status changes; reschedule tick.
		return v, tea.Batch(
			func() tea.Msg { return loadReviewView() },
			reviewPollCmd(),
		)

	case tea.KeyMsg:
		if v.viewingComments {
			return v.updateCommentKeys(msg)
		}
		return v.updateListKeys(msg)
	}
	return v, nil
}

// updateListKeys handles key events on the PR list level.
func (v reviewView) updateListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return v, pop()
	case "r":
		v.loaded = false
		v.status = ""
		return v, loadReviewView
	case "tab":
		v.tab = (v.tab + 1) % reviewTab(len(reviewTabNames))
		v.cursor = 0
		v.status = ""
		if v.tab == reviewTabInReview && len(v.tabPRs()) > 0 && !v.polling {
			v.polling = true
			return v, reviewPollCmd()
		}
	case "shift+tab":
		v.tab = (v.tab + reviewTab(len(reviewTabNames)) - 1) % reviewTab(len(reviewTabNames))
		v.cursor = 0
		v.status = ""
	case "up", "k":
		if v.cursor > 0 {
			v.cursor--
		}
	case "down", "j":
		if visible := v.tabPRs(); v.cursor < len(visible)-1 {
			v.cursor++
		}
	case "enter":
		return v.handleEnter()
	}
	return v, nil
}

// updateCommentKeys handles key events in the comment table.
func (v reviewView) updateCommentKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		v.viewingComments = false
	case "up", "k":
		if v.commentCursor > 0 {
			v.commentCursor--
		}
	case "down", "j":
		if v.commentCursor < len(v.comments)-1 {
			v.commentCursor++
		}
	case "a":
		// Toggle approved on the selected comment.
		if v.commentCursor >= len(v.comments) {
			break
		}
		c := v.comments[v.commentCursor]
		id, approved := c.ID, !c.Approved
		prid := c.PRID
		return v, func() tea.Msg {
			client.Send(client.Request{Action: "set_pr_comment_approved", CommentID: id, Approved: approved})
			resp, err := client.Send(client.Request{Action: "list_pr_comments", PRID: prid})
			if err != nil {
				return reviewCommentsMsg{err: err.Error()}
			}
			return reviewCommentsMsg{comments: resp.PRComments}
		}
	case "p":
		// Push all approved comments, then reload PR list.
		comments := v.comments
		return v, func() tea.Msg {
			for _, c := range comments {
				if c.Approved {
					client.Send(client.Request{Action: "push_pr_comment", CommentID: c.ID})
				}
			}
			return loadReviewView()
		}
	}
	return v, nil
}

// handleEnter dispatches enter based on the active tab.
func (v reviewView) handleEnter() (tea.Model, tea.Cmd) {
	visible := v.tabPRs()
	if len(visible) == 0 || v.cursor >= len(visible) {
		return v, nil
	}
	pr := visible[v.cursor]

	switch v.tab {
	case reviewTabOpen:
		id := pr.ID
		return v, func() tea.Msg {
			resp, err := client.Send(client.Request{Action: "review_start", PRID: id})
			if err != nil {
				return reviewStartedMsg{err: err.Error()}
			}
			if !resp.OK {
				return reviewStartedMsg{err: resp.Error}
			}
			return reviewStartedMsg{}
		}
	case reviewTabReady:
		id := pr.ID
		return v, func() tea.Msg {
			resp, err := client.Send(client.Request{Action: "list_pr_comments", PRID: id})
			if err != nil {
				return reviewCommentsMsg{err: err.Error()}
			}
			return reviewCommentsMsg{comments: resp.PRComments}
		}
	}
	return v, nil
}

func (v reviewView) View() string {
	var b strings.Builder

	// Tab bar.
	for i, name := range reviewTabNames {
		if reviewTab(i) == v.tab {
			b.WriteString(StyleActiveTab.Render(" [" + name + "] "))
		} else {
			b.WriteString(StyleInactiveTab.Render("  " + name + "  "))
		}
	}
	b.WriteString("\n\n")

	if !v.loaded {
		b.WriteString("  loading...\n")
		return b.String()
	}

	// Comment table view for Ready tab.
	if v.tab == reviewTabReady && v.viewingComments {
		v.renderComments(&b)
		return b.String()
	}

	// PR list.
	visible := v.tabPRs()
	if len(visible) == 0 {
		b.WriteString("  nothing here yet\n")
	} else {
		header := fmt.Sprintf("  %-6s  %-30s  %-20s  %s", "#", "TITLE", "REPO", "AUTHOR")
		b.WriteString(StyleTitle.Render(header) + "\n")
		for i, pr := range visible {
			title := pr.Title
			if len(title) > 30 {
				title = title[:27] + "..."
			}
			repo := pr.Repo
			if len(repo) > 20 {
				repo = repo[:17] + "..."
			}
			row := fmt.Sprintf("  %-6d  %-30s  %-20s  %s", pr.PRNumber, title, repo, pr.Author)
			cursor := fmt.Sprintf("> %-6d  %-30s  %-20s  %s", pr.PRNumber, title, repo, pr.Author)
			if i == v.cursor {
				b.WriteString(StyleCursor.Render(cursor) + "\n")
			} else {
				b.WriteString(row + "\n")
			}
		}
	}

	// Session log panel for In Review tab.
	if v.tab == reviewTabInReview {
		b.WriteString("\n  ── session log ──────────────────────────────────────────\n")
		if len(v.logLines) == 0 {
			b.WriteString("  (no logs)\n")
		} else {
			start := len(v.logLines) - 10
			if start < 0 {
				start = 0
			}
			for _, line := range v.logLines[start:] {
				b.WriteString("  " + line + "\n")
			}
		}
	}

	if v.status != "" {
		b.WriteString("\n")
		if strings.HasPrefix(v.status, "error") {
			b.WriteString("  " + StyleError.Render(v.status) + "\n")
		} else {
			b.WriteString("  " + StyleSuccess.Render(v.status) + "\n")
		}
	}

	return b.String()
}

// renderComments writes the comment table into b.
func (v reviewView) renderComments(b *strings.Builder) {
	header := fmt.Sprintf("  %-4s  %-12s  %-20s  %s", "[✓]", "TYPE", "FILE:LINE", "BODY")
	b.WriteString(StyleTitle.Render(header) + "\n")
	for i, c := range v.comments {
		approved := "   "
		if c.Approved {
			approved = "[✓]"
		}
		loc := fmt.Sprintf("%s:%d", c.FilePath, c.Line)
		if len(loc) > 20 {
			loc = loc[:17] + "..."
		}
		body := c.Body
		if len(body) > 50 {
			body = body[:50]
		}
		row := fmt.Sprintf("  %-4s  %-12s  %-20s  %s", approved, c.Type, loc, body)
		if i == v.commentCursor {
			b.WriteString(StyleCursor.Render(row) + "\n")
		} else {
			b.WriteString(row + "\n")
		}
	}
}
