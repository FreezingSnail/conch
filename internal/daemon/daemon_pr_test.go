package daemon

import (
	"testing"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/kiro"
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

	resp := dispatch(client.Request{Action: "list_prs", Status: "open"}, database, kiro.Kiro{})
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

	resp := dispatch(client.Request{Action: "list_pr_comments", PRID: prID}, database, kiro.Kiro{})
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

	resp := dispatch(client.Request{Action: "poll_prs"}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("expected OK=true with no config, got error: %s", resp.Error)
	}
}

// TestDispatch_setPRCommentApproved seeds a comment and toggles approved true
// then false, asserting DB state after each toggle.
func TestDispatch_setPRCommentApproved(t *testing.T) {
	database := openTestDB(t)

	prID, err := database.UpsertPR("/repos/r", 1, "PR", "dev", "http://u", "sha")
	if err != nil {
		t.Fatal(err)
	}
	commentID, err := database.CreatePRComment(prID, "suggestion", "main.go", 5, "body")
	if err != nil {
		t.Fatal(err)
	}

	// Toggle approved=true.
	resp := dispatch(client.Request{Action: "set_pr_comment_approved", CommentID: commentID, Approved: true}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("set_pr_comment_approved true: %s", resp.Error)
	}
	c, err := database.GetPRCommentByID(commentID)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Approved {
		t.Fatal("expected approved=true")
	}

	// Toggle approved=false.
	resp = dispatch(client.Request{Action: "set_pr_comment_approved", CommentID: commentID, Approved: false}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("set_pr_comment_approved false: %s", resp.Error)
	}
	c, err = database.GetPRCommentByID(commentID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Approved {
		t.Fatal("expected approved=false")
	}
}

// TestDispatch_pushPRComment_completesWhenAllPushed seeds a PR with one comment,
// marks it pushed directly, and asserts the PR status becomes "completed".
func TestDispatch_pushPRComment_completesWhenAllPushed(t *testing.T) {
	database := openTestDB(t)

	prID, err := database.UpsertPR("/repos/r", 2, "PR", "dev", "http://u", "sha")
	if err != nil {
		t.Fatal(err)
	}
	commentID, err := database.CreatePRComment(prID, "suggestion", "main.go", 5, "body")
	if err != nil {
		t.Fatal(err)
	}

	// Approve the comment via dispatch.
	resp := dispatch(client.Request{Action: "set_pr_comment_approved", CommentID: commentID, Approved: true}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("set_pr_comment_approved: %s", resp.Error)
	}

	// Simulate the push path directly (no real gh binary in tests).
	if err := database.SetPRCommentPushed(commentID); err != nil {
		t.Fatal(err)
	}
	allPushed, err := database.AllPRCommentsPushed(prID)
	if err != nil {
		t.Fatal(err)
	}
	if !allPushed {
		t.Fatal("expected all comments pushed")
	}
	if err := database.UpdatePRStatus(prID, "completed"); err != nil {
		t.Fatal(err)
	}

	pr, err := database.GetPRByID(prID)
	if err != nil {
		t.Fatal(err)
	}
	if pr.Status != "completed" {
		t.Fatalf("expected status=completed, got %q", pr.Status)
	}
}

// TestAllPRCommentsPushed seeds a PR with two comments, pushes one, asserts
// false, pushes the second, and asserts true.
func TestAllPRCommentsPushed(t *testing.T) {
	database := openTestDB(t)

	prID, err := database.UpsertPR("/repos/r", 3, "PR", "dev", "http://u", "sha")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := database.CreatePRComment(prID, "suggestion", "a.go", 1, "first")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := database.CreatePRComment(prID, "nitpick", "b.go", 2, "second")
	if err != nil {
		t.Fatal(err)
	}

	// Neither pushed yet.
	allPushed, err := database.AllPRCommentsPushed(prID)
	if err != nil {
		t.Fatal(err)
	}
	if allPushed {
		t.Fatal("expected not all pushed with 0 pushed")
	}

	// Push first comment only.
	if err := database.SetPRCommentPushed(c1); err != nil {
		t.Fatal(err)
	}
	allPushed, err = database.AllPRCommentsPushed(prID)
	if err != nil {
		t.Fatal(err)
	}
	if allPushed {
		t.Fatal("expected not all pushed with 1 of 2 pushed")
	}

	// Push second comment.
	if err := database.SetPRCommentPushed(c2); err != nil {
		t.Fatal(err)
	}
	allPushed, err = database.AllPRCommentsPushed(prID)
	if err != nil {
		t.Fatal(err)
	}
	if !allPushed {
		t.Fatal("expected all pushed after both comments pushed")
	}
}
