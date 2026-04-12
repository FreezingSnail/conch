package daemon

import (
	"strings"
	"testing"
)

// TestSynthesizeCommitSingle verifies that a single result passes through its
// type and subject unchanged, and that the body contains the original body text
// plus a "Tasks: <id>" trailer.
func TestSynthesizeCommitSingle(t *testing.T) {
	results := []taskResult{
		{taskID: 7, success: true, signal: commitSignal{Type: "feat", Subject: "add thing", Body: "details"}},
	}
	got := synthesizeCommit(results)
	if got.Type != "feat" {
		t.Errorf("type: want feat, got %s", got.Type)
	}
	if got.Subject != "add thing" {
		t.Errorf("subject: want 'add thing', got %s", got.Subject)
	}
	if !strings.Contains(got.Body, "details") {
		t.Errorf("body missing 'details': %s", got.Body)
	}
	if !strings.Contains(got.Body, "Tasks: ") {
		t.Errorf("body missing 'Tasks: ': %s", got.Body)
	}
}

// TestSynthesizeCommitMulti verifies that when two results are merged the
// highest-precedence type wins (feat > fix), both original bodies appear, and
// both task IDs are listed in the Tasks trailer.
func TestSynthesizeCommitMulti(t *testing.T) {
	results := []taskResult{
		{taskID: 1, success: true, signal: commitSignal{Type: "fix", Subject: "fix bug", Body: "body one"}},
		{taskID: 2, success: true, signal: commitSignal{Type: "feat", Subject: "add feature", Body: "body two"}},
	}
	got := synthesizeCommit(results)
	if got.Type != "feat" {
		t.Errorf("type: want feat, got %s", got.Type)
	}
	if !strings.Contains(got.Body, "body one") {
		t.Errorf("body missing 'body one': %s", got.Body)
	}
	if !strings.Contains(got.Body, "body two") {
		t.Errorf("body missing 'body two': %s", got.Body)
	}
	if !strings.Contains(got.Body, "1") || !strings.Contains(got.Body, "2") {
		t.Errorf("body missing task IDs: %s", got.Body)
	}
}

// TestSynthesizeCommitTypePrecedence verifies that among "chore", "fix", and
// "refactor" the winner is "fix" (fix has higher precedence than refactor/chore).
func TestSynthesizeCommitTypePrecedence(t *testing.T) {
	results := []taskResult{
		{taskID: 10, success: true, signal: commitSignal{Type: "chore", Subject: "s1", Body: "b1"}},
		{taskID: 11, success: true, signal: commitSignal{Type: "fix", Subject: "s2", Body: "b2"}},
		{taskID: 12, success: true, signal: commitSignal{Type: "refactor", Subject: "s3", Body: "b3"}},
	}
	got := synthesizeCommit(results)
	if got.Type != "fix" {
		t.Errorf("type: want fix, got %s", got.Type)
	}
}
