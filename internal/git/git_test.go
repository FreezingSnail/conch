package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	return dir
}

func TestDefaultBranch(t *testing.T) {
	requireGit(t)
	repo := initRepo(t)
	if b := DefaultBranch(repo); b != "main" {
		t.Fatalf("expected main, got %s", b)
	}
}

func TestWorktreeAddRemoveList(t *testing.T) {
	requireGit(t)
	repo := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt1")

	if err := WorktreeAdd(repo, wtPath, "ticket-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatal("worktree dir not created")
	}

	paths, err := WorktreeList(repo)
	if err != nil {
		t.Fatal(err)
	}
	realWt, _ := filepath.EvalSymlinks(wtPath)
	found := false
	for _, p := range paths {
		realP, _ := filepath.EvalSymlinks(p)
		if realP == realWt {
			found = true
		}
	}
	if !found {
		t.Fatalf("worktree not in list: %v", paths)
	}

	if err := WorktreeRemove(repo, wtPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wtPath); err == nil {
		t.Fatal("worktree dir should be removed")
	}
}

func TestMergeIntoMain(t *testing.T) {
	requireGit(t)
	repo := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt-merge")

	if err := WorktreeAdd(repo, wtPath, "feature"); err != nil {
		t.Fatal(err)
	}
	// Commit something on the feature branch.
	f := filepath.Join(wtPath, "file.txt")
	if err := os.WriteFile(f, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "file.txt"},
		{"commit", "-m", "add file"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = wtPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	out, err := MergeIntoMain(repo, "feature")
	if err != nil {
		t.Fatalf("MergeIntoMain: %v\n%s", err, out)
	}
}

func TestMergeIntoMainConflict(t *testing.T) {
	requireGit(t)
	repo := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt-conflict")

	if err := WorktreeAdd(repo, wtPath, "conflict-branch"); err != nil {
		t.Fatal(err)
	}
	// Commit conflicting change on feature branch.
	if err := os.WriteFile(filepath.Join(wtPath, "clash.txt"), []byte("branch"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "clash.txt"}, {"commit", "-m", "branch commit"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = wtPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	// Commit conflicting change on main.
	if err := os.WriteFile(filepath.Join(repo, "clash.txt"), []byte("main"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "clash.txt"}, {"commit", "-m", "main commit"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	out, err := MergeIntoMain(repo, "conflict-branch")
	if err == nil {
		t.Fatalf("expected conflict error, got nil; output: %s", out)
	}
}

func TestWorktreeAddIdempotent(t *testing.T) {
	requireGit(t)
	repo := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt-idem")

	if err := WorktreeAdd(repo, wtPath, "ticket-idem"); err != nil {
		t.Fatal(err)
	}
	// Second call must not error.
	if err := WorktreeAdd(repo, wtPath, "ticket-idem"); err != nil {
		t.Fatalf("second WorktreeAdd: %v", err)
	}
}
