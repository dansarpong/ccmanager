package advisor

import (
	"fmt"
	"strings"
)

// ProposedEdit is a single approvable change to a file: the new content plus a
// human-readable unified diff for review. The advisor only proposes; applying
// goes through the app's safe-write path.
type ProposedEdit struct {
	File       string
	NewContent string
	Diff       string
	Applied    bool // whether the original text was actually found and replaced
}

// ApplyRewrite produces the proposed new content for a rewrite suggestion by
// replacing the first occurrence of the original text with the suggestion. It
// does not write anything. If the original text is not found, Applied is false
// and NewContent equals the input.
func ApplyRewrite(current string, sug RewriteSuggestion) ProposedEdit {
	idx := strings.Index(current, sug.Original)
	if idx < 0 {
		return ProposedEdit{File: sug.File, NewContent: current, Applied: false}
	}
	next := current[:idx] + sug.Suggestion + current[idx+len(sug.Original):]
	return ProposedEdit{
		File:       sug.File,
		NewContent: next,
		Diff:       unifiedDiff(current, next),
		Applied:    true,
	}
}

// unifiedDiff renders a simple line-level diff between two texts. It is meant
// for human review, not for machine re-application (we keep the full new
// content for that), so it shows changed lines with context rather than exact
// hunk headers.
func unifiedDiff(oldText, newText string) string {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	// Find the common prefix and suffix so only the changed region is shown.
	start := 0
	for start < len(oldLines) && start < len(newLines) && oldLines[start] == newLines[start] {
		start++
	}
	endOld, endNew := len(oldLines), len(newLines)
	for endOld > start && endNew > start && oldLines[endOld-1] == newLines[endNew-1] {
		endOld--
		endNew--
	}

	var b strings.Builder
	const context = 2
	ctxStart := start - context
	if ctxStart < 0 {
		ctxStart = 0
	}
	for i := ctxStart; i < start; i++ {
		fmt.Fprintf(&b, "  %s\n", oldLines[i])
	}
	for i := start; i < endOld; i++ {
		fmt.Fprintf(&b, "- %s\n", oldLines[i])
	}
	for i := start; i < endNew; i++ {
		fmt.Fprintf(&b, "+ %s\n", newLines[i])
	}
	ctxEnd := endOld + context
	if ctxEnd > len(oldLines) {
		ctxEnd = len(oldLines)
	}
	for i := endOld; i < ctxEnd; i++ {
		fmt.Fprintf(&b, "  %s\n", oldLines[i])
	}
	return b.String()
}
