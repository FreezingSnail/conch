package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const singleHunkDiff = `diff --git a/foo.go b/foo.go
index 0000000..1111111 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 package main
+
+// added
 func main() {}`

const multiFileDiff = `diff --git a/a.go b/a.go
index 0000000..1111111 100644
--- a/a.go
+++ b/a.go
@@ -1,2 +1,3 @@
 package a
+// hunk1
diff --git a/b.go b/b.go
index 0000000..2222222 100644
--- a/b.go
+++ b/b.go
@@ -1,2 +1,3 @@
 package b
+// hunk2
@@ -5,2 +5,3 @@
 func b() {}
+// hunk3`

func test_parse_diff_single_hunk(t *testing.T) {
	hunks := ParseDiff(singleHunkDiff)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	h := hunks[0]
	if h.FilePath != "foo.go" {
		t.Errorf("FilePath: got %q, want %q", h.FilePath, "foo.go")
	}
	if h.HunkHeader != "@@ -1,3 +1,4 @@" {
		t.Errorf("HunkHeader: got %q", h.HunkHeader)
	}
	if len(h.Lines) == 0 {
		t.Error("expected non-empty Lines")
	}
}

func test_parse_diff_multi_file(t *testing.T) {
	hunks := ParseDiff(multiFileDiff)
	if len(hunks) != 3 {
		t.Fatalf("expected 3 hunks, got %d", len(hunks))
	}
	if hunks[0].FilePath != "a.go" {
		t.Errorf("hunk[0] FilePath: got %q", hunks[0].FilePath)
	}
	if hunks[1].FilePath != "b.go" {
		t.Errorf("hunk[1] FilePath: got %q", hunks[1].FilePath)
	}
	if hunks[2].FilePath != "b.go" {
		t.Errorf("hunk[2] FilePath: got %q", hunks[2].FilePath)
	}
}

func test_parse_diff_empty(t *testing.T) {
	hunks := ParseDiff("")
	if hunks != nil {
		t.Fatalf("expected nil, got %v", hunks)
	}
}

// initRepoWithCommit creates a temp git repo and returns (repoDir, commitHash).
func initRepoWithCommit(t *testing.T, msg string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", msg},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	hash, err := run(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	return dir, hash
}

func test_git_notes_get_set_remove(t *testing.T) {
	requireGit(t)
	repo, hash := initRepoWithCommit(t, "init")

	// No note yet — should return empty string, not error.
	note, err := NoteGet(repo, hash)
	if err != nil {
		t.Fatalf("NoteGet (empty): %v", err)
	}
	if note != "" {
		t.Errorf("expected empty note, got %q", note)
	}

	// Set a note and read it back.
	if err := NoteSet(repo, hash, "review: looks good"); err != nil {
		t.Fatalf("NoteSet: %v", err)
	}
	note, err = NoteGet(repo, hash)
	if err != nil {
		t.Fatalf("NoteGet (after set): %v", err)
	}
	if note != "review: looks good" {
		t.Errorf("NoteGet: got %q, want %q", note, "review: looks good")
	}

	// Remove the note — subsequent get should return empty.
	if err := NoteRemove(repo, hash); err != nil {
		t.Fatalf("NoteRemove: %v", err)
	}
	note, err = NoteGet(repo, hash)
	if err != nil {
		t.Fatalf("NoteGet (after remove): %v", err)
	}
	if note != "" {
		t.Errorf("expected empty note after remove, got %q", note)
	}
}

func test_log_list(t *testing.T) {
	requireGit(t)
	repo, _ := initRepoWithCommit(t, "first commit")

	// Add a second commit.
	f := filepath.Join(repo, "x.txt")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "x.txt"}, {"commit", "-m", "second commit"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	commits, err := LogList(repo)
	if err != nil {
		t.Fatalf("LogList: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	// Verify hashes are non-empty and look like abbreviated hashes.
	for i, c := range commits {
		if len(c.Hash) < 7 {
			t.Errorf("commit[%d] hash too short: %q", i, c.Hash)
		}
		if c.Subject == "" {
			t.Errorf("commit[%d] subject empty", i)
		}
	}
	// Most-recent commit is first.
	if commits[0].Subject != "second commit" {
		t.Errorf("expected most-recent first, got %q", commits[0].Subject)
	}
}

// TestDiffParseSingleHunk wraps the spec-named function for go test discovery.
func TestDiffParseSingleHunk(t *testing.T)  { test_parse_diff_single_hunk(t) }
func TestDiffParseMultiFile(t *testing.T)   { test_parse_diff_multi_file(t) }
func TestDiffParseEmpty(t *testing.T)       { test_parse_diff_empty(t) }
func TestGitNotesGetSetRemove(t *testing.T) { test_git_notes_get_set_remove(t) }
func TestLogList(t *testing.T)              { test_log_list(t) }
