package review

import (
	"os"
	"testing"
)

func TestParseReviewFile_valid(t *testing.T) {
	content := `## Review Comment

- type: suggestion
- file: pkg/foo.go
- line: 10
- body: Consider renaming this variable.

## Review Comment

- type: blocker
- file: pkg/bar.go
- line: 55
- body: This causes a nil dereference.

## Review Comment

- type: nitpick
- file: pkg/baz.go
- line: 3
- body: Missing trailing newline.
`
	f := writeTempFile(t, content)
	comments, err := ParseReviewFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(comments))
	}

	cases := []ParsedComment{
		{Type: "suggestion", FilePath: "pkg/foo.go", Line: 10, Body: "Consider renaming this variable."},
		{Type: "blocker", FilePath: "pkg/bar.go", Line: 55, Body: "This causes a nil dereference."},
		{Type: "nitpick", FilePath: "pkg/baz.go", Line: 3, Body: "Missing trailing newline."},
	}
	for i, want := range cases {
		got := comments[i]
		if got != want {
			t.Errorf("comment[%d]: got %+v, want %+v", i, got, want)
		}
	}
}

func TestParseReviewFile_malformed(t *testing.T) {
	// Second section is missing the line field — should be skipped.
	content := `## Review Comment

- type: question
- file: main.go
- line: 7
- body: Why is this exported?

## Review Comment

- type: suggestion
- file: main.go
- body: No line number here.
`
	f := writeTempFile(t, content)
	comments, err := ParseReviewFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Type != "question" || comments[0].Line != 7 {
		t.Errorf("unexpected comment: %+v", comments[0])
	}
}

func TestParseReviewFile_empty(t *testing.T) {
	f := writeTempFile(t, "")
	comments, err := ParseReviewFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(comments))
	}
}

// writeTempFile creates a temporary file with the given content and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "review-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}
