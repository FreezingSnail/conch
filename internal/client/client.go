// Package client is a thin client for the conchd Unix-socket API.
// It opens a new connection per call.
package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/FreezingSnail/conch/internal/db"
)

func sockAddr() string {
	return filepath.Join(os.Getenv("HOME"), ".conch", "daemon.sock")
}

// Request maps 1:1 to daemon actions. Unused fields are omitted from JSON via omitempty.
type Request struct {
	Action        string   `json:"action"`
	Prompt        string   `json:"prompt,omitempty"`
	Harness       string   `json:"harness,omitempty"`
	Status        string   `json:"status,omitempty"`
	TaskID        int64    `json:"task_id,omitempty"`
	TicketID      int64    `json:"ticket_id,omitempty"`
	Title         string   `json:"title,omitempty"`
	Body          string   `json:"body,omitempty"`
	BlockerID     int64    `json:"blocker_id,omitempty"`
	BlockedID     int64    `json:"blocked_id,omitempty"`
	Repo          string   `json:"repo,omitempty"`
	Description   string   `json:"description,omitempty"`
	TicketNumber  string   `json:"ticket_number,omitempty"`
	Context       string   `json:"context,omitempty"`
	Repos         []string `json:"repos,omitempty"`
	SessionID     int64    `json:"session_id,omitempty"`
	KiroSessionID string   `json:"kiro_session_id,omitempty"`
	BeforeIDs     string   `json:"before_ids,omitempty"`
	Worktree      string   `json:"worktree,omitempty"`
	Dir           string   `json:"dir,omitempty"`
	// Feedback note fields
	CommitHash string `json:"commit_hash,omitempty"`
	FilePath   string `json:"file_path,omitempty"`
	HunkHeader string `json:"hunk_header,omitempty"`
	NoteID     int64  `json:"note_id,omitempty"`
	NoteBody   string `json:"note_body,omitempty"`
	// PR review fields
	PRID      int64 `json:"pr_id,omitempty"`
	CommentID int64 `json:"comment_id,omitempty"`
	Approved  bool  `json:"approved,omitempty"`
}

// Response is the daemon's reply. OK is false and Error is set on any failure.
type Response struct {
	OK            bool                 `json:"ok"`
	Error         string               `json:"error,omitempty"`
	Sessions      []db.Session         `json:"sessions,omitempty"`
	SessionLogs   []db.SessionLog      `json:"session_logs,omitempty"`
	Tickets       []db.Ticket          `json:"tickets,omitempty"`
	Tasks         []db.Task            `json:"tasks,omitempty"`
	Task          *db.Task             `json:"task,omitempty"`
	ID            int64                `json:"id,omitempty"`
	SessionID     int64                `json:"session_id,omitempty"`
	Repos         []string             `json:"repos,omitempty"`
	Lines         []string             `json:"lines,omitempty"`
	FeedbackNotes []db.FeedbackNote    `json:"feedback_notes,omitempty"`
	PRs           []db.PullRequest     `json:"prs,omitempty"`
	PRComments    []db.PRReviewComment `json:"pr_comments,omitempty"`
}

// Send dials the socket, writes one JSON line, and reads one JSON line back.
// It returns an error only on transport failure; daemon-level errors are in Response.Error.
func Send(req Request) (Response, error) {
	conn, err := net.Dial("unix", sockAddr())
	if err != nil {
		return Response{}, fmt.Errorf("daemon not running: %w", err)
	}
	defer conn.Close()
	b, _ := json.Marshal(req)
	b = append(b, '\n')
	conn.Write(b)
	var resp Response
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		json.Unmarshal(scanner.Bytes(), &resp)
	}
	return resp, nil
}

// Ping reports whether the daemon is reachable. Safe to call when the daemon is not running.
func Ping() bool {
	resp, err := Send(Request{Action: "ping"})
	return err == nil && resp.OK
}
