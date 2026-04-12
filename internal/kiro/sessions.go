package kiro

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

// KiroDBPath returns the platform-specific path to kiro-cli's sqlite database.
func KiroDBPath() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "kiro-cli", "data.sqlite3")
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "kiro-cli", "data.sqlite3")
	}
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "kiro-cli", "data.sqlite3")
}

// FindSessionByCwd queries kiro's sqlite for the most recent session created
// in cwd after the given time. Returns the conversation UUID or empty string if
// not found yet.
func FindSessionByCwd(cwd string, after time.Time) (string, error) {
	db, err := sql.Open("sqlite", KiroDBPath()+"?mode=ro")
	if err != nil {
		return "", err
	}
	defer db.Close()
	var id string
	err = db.QueryRow(
		`SELECT conversation_id FROM conversations_v2 WHERE key=? AND created_at>? ORDER BY created_at DESC LIMIT 1`,
		cwd, after.UnixMilli(),
	).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}
