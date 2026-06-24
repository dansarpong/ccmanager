package memory

import "strings"

// stripBlockComments removes block-level HTML comments (<!-- ... -->) from
// content, mirroring what Claude Code strips before injecting a CLAUDE.md into
// context. Comments inside fenced code blocks are preserved (the docs are
// explicit about this), so the stripper tracks fence state and only removes
// comments outside fences.
//
// A comment may span multiple lines; once inside a comment we drop lines until
// the closing "-->". Text on the same line before "<!--" or after "-->" is
// kept.
func stripBlockComments(text string) string {
	var out strings.Builder
	inFence := false
	var fenceMarker string
	inComment := false

	for _, lp := range splitKeepNewline(text) {
		line := lp.body

		if !inComment {
			if marker, ok := fenceToggle(line); ok {
				if !inFence {
					inFence, fenceMarker = true, marker
				} else if marker == fenceMarker {
					inFence = false
				}
				out.WriteString(line)
				out.WriteString(lp.newline)
				continue
			}
			if inFence {
				out.WriteString(line)
				out.WriteString(lp.newline)
				continue
			}
		}

		wasInComment := inComment
		kept, stillInComment := stripCommentsFromLine(line, inComment)
		inComment = stillInComment

		// If the whole line was comment content (it produced no kept text and
		// it either started inside a comment or contained a comment marker),
		// emit nothing so the comment leaves no blank line behind. A genuinely
		// blank source line (no markers, not in a comment) is preserved.
		consumed := kept == "" && (wasInComment || strings.Contains(line, "<!--"))
		if consumed {
			continue
		}
		out.WriteString(kept)
		out.WriteString(lp.newline)
	}
	return out.String()
}

// stripCommentsFromLine removes comment spans from a single line. inComment
// says whether we begin inside an open comment; it returns the kept text and
// whether a comment is still open at end of line.
func stripCommentsFromLine(line string, inComment bool) (string, bool) {
	var out strings.Builder
	i := 0
	for i < len(line) {
		if inComment {
			end := strings.Index(line[i:], "-->")
			if end < 0 {
				return out.String(), true // comment continues to next line
			}
			i += end + len("-->")
			inComment = false
			continue
		}
		start := strings.Index(line[i:], "<!--")
		if start < 0 {
			out.WriteString(line[i:])
			return out.String(), false
		}
		out.WriteString(line[i : i+start])
		i += start + len("<!--")
		inComment = true
	}
	return out.String(), inComment
}
