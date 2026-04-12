package daemon

import (
	"testing"

	"github.com/FreezingSnail/conch/internal/client"
)

// TestDispatch_listPRs seeds two PRs with different statuses and asserts that
// list_prs with status="open" returns only the open one.
func TestDispatch_listPRs(t *testing.T) {
	database := openTestDB(t)

	// Seed an open PR and an in_review PR.
	_, err := database.UpsertPR("/repos/alpha", 1, "open PR", "alice", "http://u1", "sha1")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := database.UpsertPR("/repos/beta", 2, "in_review PR", "bob", "http://u2", "sha2")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpdatePRStatus(id2, "in_review"); err != nil {
		t.Fatal(err)
	}

	resp := dispatch(client.Request{Action: "list_prs", Status: "open"}, database)
	if !resp.OK {
		t.Fatalf("list_prs: %s", resp.Error)
	}
	if len(resp.PRs) != 1 {
		t.Fatalf("expected 1 open PR, got %d", len(resp.PRs))
	}
	if resp.PRs[0].Title != "open PR" {
		t.Fatalf("unexpected PR title: %q", resp.PRs[0].Title)
	}
}

// TestDispatch_listPRComments seeds a PR and two comments, then asserts that
// list_pr_comments returns both.
func TestDispatch_listPRComments(t *testing.T) {
	database := openTestDB(t)

	prID, err := database.UpsertPR("/repos/myrepo", 10, "my PR", "dev", "http://x", "abc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreatePRComment(prID, "suggestion", "main.go", 5, "use X"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CreatePRComment(prID, "nitpick", "util.go", 12, "rename this"); err != nil {
		t.Fatal(err)
	}

	resp := dispatch(client.Request{Action: "list_pr_comments", PRID: prID}, database)
	if !resp.OK {
		t.Fatalf("list_pr_comments: %s", resp.Error)
	}
	if len(resp.PRComments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(resp.PRComments))
	}
}

// TestDispatch_pollPRs_noConfig verifies that poll_prs returns {OK: true} when
// HOME points to an empty directory with no config file, without panicking.
func TestDispatch_pollPRs_noConfig(t *testing.T) {
	database := openTestDB(t) // sets HOME to a temp dir with no config

	resp := dispatch(client.Request{Action: "poll_prs"}, database)
	if !resp.OK {
		t.Fatalf("expected OK=true with no config, got error: %s", resp.Error)
	}
}
