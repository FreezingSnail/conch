package git

import "strings"

// DiffHunk represents a single contiguous changed region within a file.
// FilePath is the b-side path from the diff --git header.
// HunkHeader is the @@ line that opens the hunk.
// Lines contains every line that follows the @@ header until the next hunk or file.
type DiffHunk struct {
	FilePath   string
	HunkHeader string
	Lines      []string
}

// ParseDiff parses unified diff output (e.g. from git show or git diff) into
// individual hunks. Each "diff --git" line advances the current file; each "@@"
// line starts a new hunk. Lines between @@ headers are accumulated into the
// preceding hunk's Lines slice.
func ParseDiff(raw string) []DiffHunk {
	if raw == "" {
		return nil
	}
	var hunks []DiffHunk
	var currentFile string
	var current *DiffHunk

	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			// "diff --git a/foo b/foo" — extract the b-side path.
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				currentFile = strings.TrimPrefix(parts[3], "b/")
			}
		case strings.HasPrefix(line, "@@ "):
			// Flush the previous hunk and start a new one.
			if current != nil {
				hunks = append(hunks, *current)
			}
			current = &DiffHunk{FilePath: currentFile, HunkHeader: line}
		default:
			if current != nil {
				current.Lines = append(current.Lines, line)
			}
		}
	}
	if current != nil {
		hunks = append(hunks, *current)
	}
	return hunks
}
