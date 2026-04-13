// Package daemon is a Unix-socket JSON server. It receives Request messages,
// dispatches them to the db, git, and config packages, and returns Response messages.
package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/harness"
)

// SockAddr returns the canonical Unix socket path under $HOME/.conch.
func SockAddr() string {
	return filepath.Join(os.Getenv("HOME"), ".conch", "daemon.sock")
}

// Run listens on the Unix socket and blocks forever, spawning a goroutine per connection.
func Run(database *db.DB, h harness.Harness) error {
	addr := SockAddr()
	os.Remove(addr)
	ln, err := net.Listen("unix", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()
	fmt.Println("conchd: listening on", addr)
	startPRPoller(database)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go handle(conn, database, h)
	}
}

func handle(conn net.Conn, database *db.DB, h harness.Harness) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req client.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeResp(conn, client.Response{Error: "invalid json"})
			continue
		}
		writeResp(conn, dispatch(req, database, h))
	}
}

// ticketBranch returns the git branch name for a ticket. plan_setup uses
// TicketNumber as the branch; older paths use the numeric ID.
func ticketBranch(t db.Ticket) string {
	if t.TicketNumber != "" {
		return t.TicketNumber
	}
	return fmt.Sprintf("%d", t.ID)
}

// dispatch routes a request through domain handlers. Unknown actions return an
// error response rather than panicking.
func dispatch(req client.Request, database *db.DB, h harness.Harness) client.Response {
	if req.Action == "ping" {
		return client.Response{OK: true}
	}
	handlers := []func(client.Request, *db.DB, harness.Harness) (client.Response, bool){
		handleTickets,
		handleWorktrees,
		handleTasks,
		handleSessions,
		handleFeedback,
		handlePRs,
	}
	for _, fn := range handlers {
		if resp, ok := fn(req, database, h); ok {
			return resp
		}
	}
	return client.Response{Error: "unknown action"}
}

func writeResp(w io.Writer, r client.Response) {
	b, _ := json.Marshal(r)
	b = append(b, '\n')
	w.Write(b)
}
