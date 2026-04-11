// Package git provides thin wrappers around the git CLI for worktree management.
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// DefaultBranch checks for a refs/heads/main file and returns "main" if present,
// otherwise "master". Does not query the remote.
func DefaultBranch(repoPath string) string {
	if _, err := os.Stat(filepath.Join(repoPath, ".git", "refs", "heads", "main")); err == nil {
		return "main"
	}
	return "master"
}

// WorktreeAdd creates a new branch named branch and checks it out at worktreePath.
// It is idempotent: if the worktree is already registered it returns nil; if the
// path exists on disk but is unregistered it is removed before recreating; if the
// branch already exists the -b flag is omitted.
func WorktreeAdd(repoPath, worktreePath, branch string) error {
	// Already registered — nothing to do.
	paths, err := WorktreeList(repoPath)
	if err == nil {
		realTarget, _ := filepath.EvalSymlinks(worktreePath)
		for _, p := range paths {
			realP, _ := filepath.EvalSymlinks(p)
			if realP == realTarget || p == worktreePath {
				return nil
			}
		}
	}
	// Path exists on disk but not registered — remove it so git is happy.
	if _, err := os.Stat(worktreePath); err == nil {
		if err := os.RemoveAll(worktreePath); err != nil {
			return err
		}
	}
	base := DefaultBranch(repoPath)
	if _, err := run(repoPath, "worktree", "add", "-b", branch, worktreePath, base); err != nil {
		// Branch already exists; check out without creating it.
		_, err = run(repoPath, "worktree", "add", worktreePath, branch)
		return err
	}
	return nil
}

// WorktreeRemove removes the worktree at worktreePath.
func WorktreeRemove(repoPath, worktreePath string) error {
	_, err := run(repoPath, "worktree", "remove", "--force", worktreePath)
	return err
}

// WorktreeList returns the list of worktree paths for the repo.
func WorktreeList(repoPath string) ([]string, error) {
	out, err := run(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			paths = append(paths, strings.TrimPrefix(line, "worktree "))
		}
	}
	return paths, nil
}

// FetchMain fetches only the default branch from origin, not all refs.
func FetchMain(repoPath string) error {
	branch := DefaultBranch(repoPath)
	_, err := run(repoPath, "fetch", "origin", branch)
	return err
}

// RebaseOntoMain rebases the worktree's current branch onto origin/<default>.
// worktreePath (not repoPath) is used as the working directory.
func RebaseOntoMain(repoPath, worktreePath string) error {
	branch := DefaultBranch(repoPath)
	_, err := run(worktreePath, "rebase", "origin/"+branch)
	return err
}

// Push pushes the given branch to origin.
func Push(worktreePath, branch string) error {
	_, err := run(worktreePath, "push", "--set-upstream", "origin", branch)
	return err
}

// Commit holds the hash and subject line of a single git commit.
type Commit struct {
	Hash    string
	Subject string
}

// LogList returns commits reachable from HEAD in worktreePath, excluding merges.
// Each entry is parsed from `git log --oneline --no-merges` output.
func LogList(worktreePath string) ([]Commit, error) {
	out, err := run(worktreePath, "log", "--oneline", "--no-merges")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	commits := make([]Commit, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format: "<hash> <subject>"
		idx := strings.IndexByte(line, ' ')
		if idx < 0 {
			continue
		}
		commits = append(commits, Commit{Hash: line[:idx], Subject: line[idx+1:]})
	}
	return commits, nil
}

// DiffCommit returns the unified diff for a single commit (git show --unified=3).
func DiffCommit(worktreePath, hash string) (string, error) {
	return run(worktreePath, "show", "--unified=3", hash)
}

// NoteGet returns the git note attached to hash, or "" if none exists.
func NoteGet(worktreePath, hash string) (string, error) {
	out, err := run(worktreePath, "notes", "show", hash)
	if err != nil {
		// exit status 1 means no note — treat as empty, not an error.
		return "", nil
	}
	return out, nil
}

// NoteSet attaches content as a git note on hash, overwriting any existing note.
func NoteSet(worktreePath, hash, content string) error {
	_, err := run(worktreePath, "notes", "add", "-f", "-m", content, hash)
	return err
}

// NoteRemove removes the git note attached to hash.
func NoteRemove(worktreePath, hash string) error {
	_, err := run(worktreePath, "notes", "remove", hash)
	return err
}

// MergeIntoMain checks out the default branch in repoPath and merges branch into it.
// Returns the combined git output and any error. On a merge conflict the repo is
// left in the conflicted state and a non-nil error is returned alongside the output.
func MergeIntoMain(repoPath, branch string) (string, error) {
	main := DefaultBranch(repoPath)
	if _, err := run(repoPath, "checkout", main); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "merge", branch)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
