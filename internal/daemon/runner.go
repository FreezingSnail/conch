package daemon

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/kiro"
)

// runBackground streams stdout line-by-line into session_logs and updates the
// session status to "completed" or "error" when the process exits.
func runBackground(sessionID int64, prompt string, database *db.DB) {
	cmd := kiro.Kiro{}.Background(prompt)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		return
	}
	combined := io.MultiReader(stdout, stderr)
	if err := cmd.Start(); err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		database.AppendSessionLog(sessionID, "error", err.Error())
		return
	}
	scanner := bufio.NewScanner(combined)
	for scanner.Scan() {
		database.AppendSessionLog(sessionID, "stdout", scanner.Text())
	}
	if err := cmd.Wait(); err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		database.AppendSessionLog(sessionID, "exit_error", err.Error())
		return
	}
	database.UpdateSessionStatus(sessionID, "completed")
}

// runExecutor spawns a headless kiro executor in the ticket's worktree and
// updates the session status when the process exits.
func runExecutor(sessionID int64, prompt, worktreePath string, database *db.DB) {
	cmd := kiro.Kiro{}.BackgroundWithAgent("executor", prompt, worktreePath)
	// Ensure conch binary is findable. The daemon may be launched without the
	// user's full PATH. Prepend the Go bin dir and the daemon's own dir.
	goBin := filepath.Join(os.Getenv("HOME"), "go", "bin")
	selfDir := filepath.Dir(os.Args[0])
	cmd.Env = append(os.Environ(), "PATH="+goBin+":"+selfDir+":"+os.Getenv("PATH"))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		return
	}
	combined := io.MultiReader(stdout, stderr)
	launchTime := time.Now()
	if err := cmd.Start(); err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		database.AppendSessionLog(sessionID, "error", err.Error())
		return
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if uuid, err := kiro.FindSessionByCwd(worktreePath, launchTime); err == nil && uuid != "" {
					database.SetSessionKiroID(sessionID, uuid) //nolint:errcheck
					return
				}
			case <-stop:
				return
			}
		}
	}()
	scanner := bufio.NewScanner(combined)
	for scanner.Scan() {
		database.AppendSessionLog(sessionID, "stdout", scanner.Text())
	}
	if err := cmd.Wait(); err != nil {
		database.UpdateSessionStatus(sessionID, "error")
		database.AppendSessionLog(sessionID, "exit_error", err.Error())
	} else {
		database.UpdateSessionStatus(sessionID, "completed")
	}
	close(stop)
}
