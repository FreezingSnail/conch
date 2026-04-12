package kiro

import (
	"strings"
	"testing"

	"github.com/FreezingSnail/conch/internal/db"
)

// test_build_replan_prompt_no_notes: zero notes → header present, no "Additional context".
func test_build_replan_prompt_no_notes(t *testing.T) {
	got := BuildReplanPrompt("T-1", 42, "", "", nil)
	if !strings.Contains(got, "[CONCH PLANNING]") {
		t.Fatalf("expected [CONCH PLANNING] header, got: %q", got)
	}
	if strings.Contains(got, "Additional context") {
		t.Fatalf("expected no Additional context section, got: %q", got)
	}
}

// test_build_replan_prompt_with_notes: one note → filePath, hunkHeader, and body present.
func test_build_replan_prompt_with_notes(t *testing.T) {
	notes := []db.FeedbackNote{
		{FilePath: "main.go", HunkHeader: "@@ -1,3 +1,4 @@", Body: "fix the loop"},
	}
	got := BuildReplanPrompt("T-2", 7, "My Title", "my idea", notes)
	for _, want := range []string{"main.go", "@@ -1,3 +1,4 @@", "fix the loop", "My Title", "my idea"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in prompt, got: %q", want, got)
		}
	}
}

// test_build_replan_prompt_multiple_notes: multiple notes → all present in output.
func test_build_replan_prompt_multiple_notes(t *testing.T) {
	notes := []db.FeedbackNote{
		{FilePath: "a.go", HunkHeader: "@@ -1 @@", Body: "note one"},
		{FilePath: "b.go", HunkHeader: "@@ -2 @@", Body: "note two"},
		{FilePath: "c.go", HunkHeader: "@@ -3 @@", Body: "note three"},
	}
	got := BuildReplanPrompt("T-3", 99, "", "", notes)
	for _, want := range []string{"a.go", "b.go", "c.go", "note one", "note two", "note three"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in prompt, got: %q", want, got)
		}
	}
}

// test_build_prompt_with_title_and_idea: title and idea appear in output.
func test_build_prompt_with_title_and_idea(t *testing.T) {
	got := BuildPrompt("T-4", 5, "Add login", "users need to log in with email")
	for _, want := range []string{"[CONCH PLANNING]", "ticket:T-4", "id:5", "Add login", "users need to log in"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in prompt, got: %q", want, got)
		}
	}
}

// test_build_prompt_empty_title_idea: empty title/idea → no extra lines.
func test_build_prompt_empty_title_idea(t *testing.T) {
	got := BuildPrompt("T-5", 6, "", "")
	if strings.Contains(got, "title:") || strings.Contains(got, "idea:") {
		t.Fatalf("expected no title/idea lines, got: %q", got)
	}
}

func TestBuildReplanPromptNoNotes(t *testing.T)       { test_build_replan_prompt_no_notes(t) }
func TestBuildReplanPromptWithNotes(t *testing.T)     { test_build_replan_prompt_with_notes(t) }
func TestBuildReplanPromptMultipleNotes(t *testing.T) { test_build_replan_prompt_multiple_notes(t) }
func TestBuildPromptWithTitleAndIdea(t *testing.T)    { test_build_prompt_with_title_and_idea(t) }
func TestBuildPromptEmptyTitleIdea(t *testing.T)      { test_build_prompt_empty_title_idea(t) }
