package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// test_DefaultBranch_packedRefs verifies DefaultBranch returns "main" when the
// loose ref has been packed into .git/packed-refs and removed from refs/heads/.
func test_DefaultBranch_packedRefs(t *testing.T) {
	requireGit(t)
	repo := initRepo(t) // initialises with main branch

	// Pack all refs then remove the loose ref so only packed-refs remains.
	cmd := exec.Command("git", "pack-refs", "--all")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pack-refs: %s", out)
	}
	loosePath := filepath.Join(repo, ".git", "refs", "heads", "main")
	if err := os.Remove(loosePath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove loose ref: %v", err)
	}

	if b := DefaultBranch(repo); b != "main" {
		t.Fatalf("expected main, got %s", b)
	}
}

// test_FilesChanged commits a known file and asserts FilesChanged returns it.
func test_FilesChanged(t *testing.T) {
	requireGit(t)
	repo, _ := initRepoWithCommit(t, "init")

	f := filepath.Join(repo, "hello.txt")
	if err := os.WriteFile(f, []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "hello.txt"}, {"commit", "-m", "add hello"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	hash, err := run(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}

	files, err := FilesChanged(repo, hash)
	if err != nil {
		t.Fatalf("FilesChanged: %v", err)
	}
	if len(files) != 1 || files[0] != "hello.txt" {
		t.Fatalf("expected [hello.txt], got %v", files)
	}
}

// test_CommitMessage commits with a known message and asserts CommitMessage contains it.
func test_CommitMessage(t *testing.T) {
	requireGit(t)
	repo, hash := initRepoWithCommit(t, "my unique message")

	msg, err := CommitMessage(repo, hash)
	if err != nil {
		t.Fatalf("CommitMessage: %v", err)
	}
	if !contains(msg, "my unique message") {
		t.Fatalf("expected message to contain %q, got %q", "my unique message", msg)
	}
}

// contains is a simple substring check to avoid importing strings in test file.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

func TestDefaultBranch_packedRefs(t *testing.T) { test_DefaultBranch_packedRefs(t) }
func TestFilesChanged(t *testing.T)             { test_FilesChanged(t) }
func TestCommitMessage(t *testing.T)            { test_CommitMessage(t) }
