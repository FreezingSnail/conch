// Package review parses .conch-review.md files produced by the pr-reviewer agent.
package review

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// ParsedComment holds one review comment parsed from a .conch-review.md file.
type ParsedComment struct {
	Type     string // suggestion | nitpick | blocker | question
	FilePath string
	Line     int
	Body     string
}

// ParseReviewFile reads the file at path and returns all valid ## Review Comment
// sections. Sections missing required fields or with an unparseable line number
// are skipped silently.
func ParseReviewFile(path string) ([]ParsedComment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var comments []ParsedComment
	// fields for the section currently being accumulated
	var typ, filePath, body string
	var line int
	inSection := false
	lineOK := false

	// flush finalises the current section if all required fields are present.
	flush := func() {
		if inSection && typ != "" && filePath != "" && body != "" && lineOK {
			comments = append(comments, ParsedComment{
				Type:     typ,
				FilePath: filePath,
				Line:     line,
				Body:     body,
			})
		}
		typ, filePath, body = "", "", ""
		line = 0
		lineOK = false
		inSection = false
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()

		if text == "## Review Comment" {
			flush()
			inSection = true
			continue
		}

		if !inSection {
			continue
		}

		// Parse bullet fields: "- key: value"
		if !strings.HasPrefix(text, "- ") {
			continue
		}
		rest := text[2:]
		idx := strings.Index(rest, ": ")
		if idx < 0 {
			continue
		}
		key := rest[:idx]
		val := rest[idx+2:]

		switch key {
		case "type":
			typ = val
		case "file":
			filePath = val
		case "line":
			n, err := strconv.Atoi(val)
			if err == nil {
				line = n
				lineOK = true
			}
		case "body":
			body = val
		}
	}
	flush() // handle the last section

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return comments, nil
}
