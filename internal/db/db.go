// Package db is the SQLite-backed store for tickets, tasks, sessions, and session logs.
// All data lives at ~/.conch/conch.db.
package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a sql.DB connection to conch.db. Obtain via Open.
type DB struct {
	conn *sql.DB
}

// Open creates ~/.conch if missing, opens the SQLite database, and runs migrations.
// Safe to call on an existing database.
func Open() (*DB, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".conch")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	conn, err := sql.Open("sqlite", filepath.Join(dir, "conch.db"))
	if err != nil {
		return nil, err
	}
	d := &DB{conn: conn}
	return d, d.migrate()
}

// migrate creates the schema on first run. Additive column migrations silently
// ignore errors because the column may already exist in an older database.
func (d *DB) migrate() error {
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS tickets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'open',
			dependencies TEXT,
			repo TEXT,
			worktree_path TEXT,
			created_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'todo',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS task_deps (
			blocker_id INTEGER NOT NULL,
			blocked_id INTEGER NOT NULL,
			PRIMARY KEY (blocker_id, blocked_id)
		);
		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER,
			task_id INTEGER,
			harness TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'running',
			started_at DATETIME NOT NULL,
			ended_at DATETIME
		);
		CREATE TABLE IF NOT EXISTS session_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			event TEXT NOT NULL,
			payload TEXT,
			created_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS worktrees (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ticket_id INTEGER NOT NULL,
			repo TEXT NOT NULL,
			worktree_path TEXT NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	// Additive migrations for existing DBs — ignore errors when column already exists.
	for _, stmt := range []string{
		`ALTER TABLE tickets ADD COLUMN repo TEXT`,
		`ALTER TABLE tickets ADD COLUMN worktree_path TEXT`,
		`ALTER TABLE tickets ADD COLUMN ticket_number TEXT`,
		`ALTER TABLE sessions ADD COLUMN kiro_session_id TEXT`,
	} {
		d.conn.Exec(stmt)
	}
	return nil
}

func (d *DB) Close() error { return d.conn.Close() }

// Tickets

// Ticket represents a unit of work, optionally linked to a git repo and worktree.
type Ticket struct {
	ID           int64
	TicketNumber string // Free-text identifier, e.g. "PROJ-123".
	Title        string
	Description  string
	Status       string
	Dependencies string // Legacy field; unused.
	Repo         string
	WorktreePath string // Empty when no worktree is active for this ticket.
	CreatedAt    time.Time
}

// CreateTicket inserts a new ticket and returns its auto-increment ID.
func (d *DB) CreateTicket(ticketNumber, title, description, repo string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO tickets (ticket_number, title, description, status, repo, created_at) VALUES (?, ?, ?, 'open', ?, ?)`,
		ticketNumber, title, description, repo, time.Now(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Worktree records a single git worktree created for a ticket in a specific repo.
type Worktree struct {
	ID           int64
	TicketID     int64
	Repo         string
	WorktreePath string
}

// CreateWorktree records a worktree for the given ticket and repo.
func (d *DB) CreateWorktree(ticketID int64, repo, path string) error {
	_, err := d.conn.Exec(
		`INSERT INTO worktrees (ticket_id, repo, worktree_path) VALUES (?, ?, ?)`,
		ticketID, repo, path,
	)
	return err
}

// ListWorktreesByTicket returns all worktrees associated with a ticket.
func (d *DB) ListWorktreesByTicket(ticketID int64) ([]Worktree, error) {
	rows, err := d.conn.Query(
		`SELECT id, ticket_id, repo, worktree_path FROM worktrees WHERE ticket_id=?`, ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var wts []Worktree
	for rows.Next() {
		var w Worktree
		if err := rows.Scan(&w.ID, &w.TicketID, &w.Repo, &w.WorktreePath); err != nil {
			return nil, err
		}
		wts = append(wts, w)
	}
	return wts, nil
}

// DeleteTicket hard-deletes a ticket and all associated worktrees rows.
func (d *DB) DeleteTicket(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM worktrees WHERE ticket_id=?`, id)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(`DELETE FROM tickets WHERE id=?`, id)
	return err
}

// DeleteWorktreeByPath removes the worktrees row with the given path.
func (d *DB) DeleteWorktreeByPath(path string) error {
	_, err := d.conn.Exec(`DELETE FROM worktrees WHERE worktree_path=?`, path)
	return err
}

// SetSessionKiroID stores the kiro-cli session UUID on an existing session row.
func (d *DB) SetSessionKiroID(sessionID int64, kiroSessionID string) error {
	_, err := d.conn.Exec(`UPDATE sessions SET kiro_session_id=? WHERE id=?`, kiroSessionID, sessionID)
	return err
}

func (d *DB) SetTicketRepo(id int64, repo, worktreePath string) error {
	_, err := d.conn.Exec(`UPDATE tickets SET repo=?, worktree_path=? WHERE id=?`, repo, worktreePath, id)
	return err
}

// ListTickets returns all tickets ordered by creation time descending.
func (d *DB) ListTickets() ([]Ticket, error) {
	rows, err := d.conn.Query(`SELECT id, COALESCE(ticket_number,''), title, description, status, COALESCE(dependencies,''), COALESCE(repo,''), COALESCE(worktree_path,''), created_at FROM tickets ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.TicketNumber, &t.Title, &t.Description, &t.Status, &t.Dependencies, &t.Repo, &t.WorktreePath, &t.CreatedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, nil
}

// GetTicketByID returns the ticket with the given ID. Returns sql.ErrNoRows if not found.
func (d *DB) GetTicketByID(id int64) (Ticket, error) {
	var t Ticket
	err := d.conn.QueryRow(
		`SELECT id, COALESCE(ticket_number,''), title, description, status, COALESCE(dependencies,''), COALESCE(repo,''), COALESCE(worktree_path,''), created_at FROM tickets WHERE id=?`, id,
	).Scan(&t.ID, &t.TicketNumber, &t.Title, &t.Description, &t.Status, &t.Dependencies, &t.Repo, &t.WorktreePath, &t.CreatedAt)
	return t, err
}

// Tasks

// Task is a discrete step within a Ticket.
type Task struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticket_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateTask creates a task with an empty body. Delegates to CreateTaskWithBody.
func (d *DB) CreateTask(ticketID int64, title string) (int64, error) {
	return d.CreateTaskWithBody(ticketID, title, "")
}

func (d *DB) CreateTaskWithBody(ticketID int64, title, body string) (int64, error) {
	now := time.Now()
	res, err := d.conn.Exec(
		`INSERT INTO tasks (ticket_id, title, body, status, created_at, updated_at) VALUES (?, ?, ?, 'todo', ?, ?)`,
		ticketID, title, body, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetTask returns the task with the given ID. Returns sql.ErrNoRows if not found.
func (d *DB) GetTask(id int64) (Task, error) {
	var t Task
	err := d.conn.QueryRow(
		`SELECT id, ticket_id, title, body, status, created_at, updated_at FROM tasks WHERE id=?`, id,
	).Scan(&t.ID, &t.TicketID, &t.Title, &t.Body, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (d *DB) UpdateTaskStatus(id int64, status string) error {
	_, err := d.conn.Exec(`UPDATE tasks SET status=?, updated_at=? WHERE id=?`, status, time.Now(), id)
	return err
}

func (d *DB) ListTasksByTicket(ticketID int64) ([]Task, error) {
	rows, err := d.conn.Query(
		`SELECT id, ticket_id, title, body, status, created_at, updated_at FROM tasks WHERE ticket_id=? ORDER BY created_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (d *DB) AddDependency(blockerID, blockedID int64) error {
	_, err := d.conn.Exec(`INSERT OR IGNORE INTO task_deps (blocker_id, blocked_id) VALUES (?, ?)`, blockerID, blockedID)
	return err
}

func (d *DB) RemoveDependency(blockerID, blockedID int64) error {
	_, err := d.conn.Exec(`DELETE FROM task_deps WHERE blocker_id=? AND blocked_id=?`, blockerID, blockedID)
	return err
}

// ListBlockedBy returns the tasks that must complete before taskID can start.
func (d *DB) ListBlockedBy(taskID int64) ([]Task, error) {
	rows, err := d.conn.Query(`
		SELECT t.id, t.ticket_id, t.title, t.body, t.status, t.created_at, t.updated_at
		FROM tasks t JOIN task_deps d ON t.id = d.blocker_id
		WHERE d.blocked_id = ?`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

// ListBlocks returns the tasks that taskID is currently blocking.
func (d *DB) ListBlocks(taskID int64) ([]Task, error) {
	rows, err := d.conn.Query(`
		SELECT t.id, t.ticket_id, t.title, t.body, t.status, t.created_at, t.updated_at
		FROM tasks t JOIN task_deps d ON t.id = d.blocked_id
		WHERE d.blocker_id = ?`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func scanTasks(rows *sql.Rows) ([]Task, error) {
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.TicketID, &t.Title, &t.Body, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// Sessions

// Session records a harness invocation, either interactive or background.
type Session struct {
	ID        int64
	TicketID  int64
	TaskID    int64
	Harness   string
	Status    string
	StartedAt time.Time
	EndedAt   *time.Time // Nil while the session is still running.
}

// CreateSession inserts a new session record. Pass ticketID=0 when not linked to a ticket.
func (d *DB) CreateSession(ticketID int64, harness, status string) (int64, error) {
	var res sql.Result
	var err error
	if ticketID == 0 {
		res, err = d.conn.Exec(
			`INSERT INTO sessions (harness, status, started_at) VALUES (?, ?, ?)`,
			harness, status, time.Now(),
		)
	} else {
		res, err = d.conn.Exec(
			`INSERT INTO sessions (ticket_id, harness, status, started_at) VALUES (?, ?, ?, ?)`,
			ticketID, harness, status, time.Now(),
		)
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateSessionStatus(id int64, status string) error {
	_, err := d.conn.Exec(`UPDATE sessions SET status=?, ended_at=? WHERE id=?`, status, time.Now(), id)
	return err
}

func (d *DB) ListSessions() ([]Session, error) {
	rows, err := d.conn.Query(`SELECT id, COALESCE(ticket_id,0), COALESCE(task_id,0), harness, status, started_at, ended_at FROM sessions ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.TicketID, &s.TaskID, &s.Harness, &s.Status, &s.StartedAt, &s.EndedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (d *DB) AppendSessionLog(sessionID int64, event, payload string) error {
	_, err := d.conn.Exec(
		`INSERT INTO session_logs (session_id, event, payload, created_at) VALUES (?, ?, ?, ?)`,
		sessionID, event, payload, time.Now(),
	)
	return err
}
