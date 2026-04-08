package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

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

type Ticket struct {
	ID           int64
	Title        string
	Description  string
	Status       string
	Dependencies string
	Repo         string
	WorktreePath string
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

// Tasks

type Task struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticket_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

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

// ListBlockedBy returns tasks that must complete before taskID can start.
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

// ListBlocks returns tasks that taskID is blocking.
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

type Session struct {
	ID        int64
	TicketID  int64
	TaskID    int64
	Harness   string
	Status    string
	StartedAt time.Time
	EndedAt   *time.Time
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
