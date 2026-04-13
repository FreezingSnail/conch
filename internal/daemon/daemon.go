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

	"github.com/FreezingSnail/conch/internal/assets"
	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/db"
)

// slugPreamble returns the slug mode preamble for the given intensity level.
func slugPreamble(mode string) string {
	rules := "Respond terse like smart slug. All technical substance stay. Only fluff die. " +
		"Drop articles, filler (just/really/basically/actually/simply), pleasantries, hedging. " +
		"Fragments permitted. Short synonyms. Technical terms exact. Code blocks normal. " +
		"This mode MUST remain active every response."
	switch mode {
	case "lite":
		rules += " Keep articles and full sentences. Professional but tight."
	case "slugineer":
		rules += " Abbreviate (DB/auth/config/req/res/fn/impl). Strip conjunctions. Use arrows for causality (X → Y). One word when sufficient."
	}
	return "## Communication Style\n\nSlug mode: " + mode + ". " + rules + "\n\n"
}

// injectSlugMode prepends a slug activation preamble to the agent's prompt field.
// For the planning agent, slugdd skill content is also injected between the slug
// preamble and the existing prompt. Returns b unchanged if mode is "off" or JSON
// cannot be parsed.
func injectSlugMode(b []byte, mode string) []byte {
	if mode == "off" {
		return b
	}
	var agent map[string]interface{}
	if err := json.Unmarshal(b, &agent); err != nil {
		return b
	}
	prefix := slugPreamble(mode)
	if agent["name"] == "planning" {
		prefix += string(assets.SlugddSkill) + "\n\n"
	}
	if p, ok := agent["prompt"].(string); ok {
		agent["prompt"] = prefix + p
	}
	out, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return b
	}
	return out
}

// seedKiroConfig writes embedded kiro config files into the worktree.
// Non-fatal: worktree is still usable without them.
func seedKiroConfig(worktreePath, slugMode string) {
	settingsDir := filepath.Join(worktreePath, ".kiro", "settings")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return
	}
	os.WriteFile(filepath.Join(settingsDir, "lsp.json"), assets.KiroLSPConfig, 0o644) //nolint:errcheck

	agentsDir := filepath.Join(worktreePath, ".kiro", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return
	}
	os.WriteFile(filepath.Join(agentsDir, "implementor.json"), injectSlugMode(assets.KiroImplementorAgent, slugMode), 0o644) //nolint:errcheck
	os.WriteFile(filepath.Join(agentsDir, "planning.json"), injectSlugMode(assets.KiroPlanningAgent, slugMode), 0o644)       //nolint:errcheck
	os.WriteFile(filepath.Join(agentsDir, "default.json"), injectSlugMode(assets.KiroDefaultAgent, slugMode), 0o644)         //nolint:errcheck
	os.WriteFile(filepath.Join(agentsDir, "pr-reviewer.json"), injectSlugMode(assets.KiroPRReviewerAgent, slugMode), 0o644)  //nolint:errcheck

	for name, data := range map[string][]byte{"slug": assets.SlugSkill, "slugdd": assets.SlugddSkill, "CONCH_TASK": assets.ConchTaskSkill} {
		dir := filepath.Join(worktreePath, ".kiro", "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		os.WriteFile(filepath.Join(dir, "SKILL.md"), data, 0o644) //nolint:errcheck
	}
}

// SockAddr returns the canonical Unix socket path under $HOME/.conch.
func SockAddr() string {
	return filepath.Join(os.Getenv("HOME"), ".conch", "daemon.sock")
}

// Run listens on the Unix socket and blocks forever, spawning a goroutine per connection.
func Run(database *db.DB) error {
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
		go handle(conn, database)
	}
}

func handle(conn net.Conn, database *db.DB) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req client.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			writeResp(conn, client.Response{Error: "invalid json"})
			continue
		}
		writeResp(conn, dispatch(req, database))
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
func dispatch(req client.Request, database *db.DB) client.Response {
	if req.Action == "ping" {
		return client.Response{OK: true}
	}
	handlers := []func(client.Request, *db.DB) (client.Response, bool){
		handleTickets,
		handleWorktrees,
		handleTasks,
		handleSessions,
		handleFeedback,
		handlePRs,
	}
	for _, h := range handlers {
		if resp, ok := h(req, database); ok {
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
