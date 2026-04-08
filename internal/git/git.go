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

// DefaultBranch returns "main" if it exists, otherwise "master".
func DefaultBranch(repoPath string) string {
	if _, err := os.Stat(filepath.Join(repoPath, ".git", "refs", "heads", "main")); err == nil {
		return "main"
	}
	return "master"
}

// WorktreeAdd creates a new worktree at worktreePath on a new branch named branch.
func WorktreeAdd(repoPath, worktreePath, branch string) error {
	_, err := run(repoPath, "worktree", "add", "-b", branch, worktreePath, DefaultBranch(repoPath))
	return err
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

// FetchMain fetches the default branch from origin.
func FetchMain(repoPath string) error {
	branch := DefaultBranch(repoPath)
	_, err := run(repoPath, "fetch", "origin", branch)
	return err
}

// RebaseOntoMain rebases the worktree's current branch onto origin/<default>.
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
