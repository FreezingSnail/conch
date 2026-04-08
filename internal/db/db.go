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
	`)
	if err != nil {
		return err
	}
	// Additive migrations for existing DBs
	for _, col := range []string{
		`ALTER TABLE tickets ADD COLUMN repo TEXT`,
		`ALTER TABLE tickets ADD COLUMN worktree_path TEXT`,
	} {
		d.conn.Exec(col) // ignore error — column may already exist
	}
	return nil
}

func (d *DB) Close() error { return d.conn.Close() }

// Tickets

// Ticket represents a unit of work, optionally linked to a git repo and worktree.
type Ticket struct {
	ID           int64
	Title        string
	Description  string
	Status       string
	Dependencies string // Legacy field; unused.
	Repo         string
	WorktreePath string // Empty when no worktree is active for this ticket.
	CreatedAt    time.Time
}

func (d *DB) CreateTicket(title, description, repo string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO tickets (title, description, status, repo, created_at) VALUES (?, ?, 'open', ?, ?)`,
		title, description, repo, time.Now(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) SetTicketRepo(id int64, repo, worktreePath string) error {
	_, err := d.conn.Exec(`UPDATE tickets SET repo=?, worktree_path=? WHERE id=?`, repo, worktreePath, id)
	return err
}

func (d *DB) ListTickets() ([]Ticket, error) {
	rows, err := d.conn.Query(`SELECT id, title, description, status, COALESCE(dependencies,''), COALESCE(repo,''), COALESCE(worktree_path,''), created_at FROM tickets ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Dependencies, &t.Repo, &t.WorktreePath, &t.CreatedAt); err != nil {
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
		`SELECT id, title, description, status, COALESCE(dependencies,''), COALESCE(repo,''), COALESCE(worktree_path,''), created_at FROM tickets WHERE id=?`, id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Dependencies, &t.Repo, &t.WorktreePath, &t.CreatedAt)
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

func (d *DB) CreateSession(harness, status string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO sessions (harness, status, started_at) VALUES (?, ?, ?)`,
		harness, status, time.Now(),
	)
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
