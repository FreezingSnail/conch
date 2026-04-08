package tui

import "strings"

// wrapInput renders a labelled text input with word-wrap at the given terminal
// width. The cursor glyph is appended to the last visible line. width <= 0
// disables wrapping (single line).
func wrapInput(label, value string, width int) string {
	line := label + value + "█"
	if width <= 0 || len(line) <= width {
		return "  " + line
	}
	// Available content width after the two-space indent.
	avail := width - 2
	if avail < 1 {
		avail = 1
	}
	// Wrap on word boundaries where possible.
	words := strings.Fields(line)
	var lines []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if len(cur)+1+len(w) <= avail {
			cur += " " + w
		} else {
			lines = append(lines, "  "+cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, "  "+cur)
	}
	return strings.Join(lines, "\n")
}
