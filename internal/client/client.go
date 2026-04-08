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

type Request struct {
	Action      string `json:"action"`
	Prompt      string `json:"prompt,omitempty"`
	Harness     string `json:"harness,omitempty"`
	Status      string `json:"status,omitempty"`
	TaskID      int64  `json:"task_id,omitempty"`
	TicketID    int64  `json:"ticket_id,omitempty"`
	Title       string `json:"title,omitempty"`
	Body        string `json:"body,omitempty"`
	BlockerID   int64  `json:"blocker_id,omitempty"`
	BlockedID   int64  `json:"blocked_id,omitempty"`
	Repo        string `json:"repo,omitempty"`
	Description string `json:"description,omitempty"`
}

type Response struct {
	OK       bool         `json:"ok"`
	Error    string       `json:"error,omitempty"`
	Sessions []db.Session `json:"sessions,omitempty"`
	Tickets  []db.Ticket  `json:"tickets,omitempty"`
	Tasks    []db.Task    `json:"tasks,omitempty"`
	Task     *db.Task     `json:"task,omitempty"`
	ID       int64        `json:"id,omitempty"`
	Repos    []string     `json:"repos,omitempty"`
	Lines    []string     `json:"lines,omitempty"`
}

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

func Ping() bool {
	resp, err := Send(Request{Action: "ping"})
	return err == nil && resp.OK
}
