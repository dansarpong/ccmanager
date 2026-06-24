package advisor

import (
	"fmt"
	"strings"

	"ccmanager/internal/memory"
)

// vagueWords are weak instruction words that tend to produce inconsistent
// behaviour ("format code properly" vs "use 2-space indentation").
var vagueWords = []string{
	"properly", "correctly", "appropriate", "appropriately",
	"clean", "cleanly", "nice", "nicely", "good", "well",
	"reasonable", "sensible", "as needed", "etc",
}

// maxLines is the soft cap above which a CLAUDE.md is flagged for trimming.
const maxLines = 200

// Lint runs all static checks over a resolved memory stack and returns the
// findings. It is pure and offline.
func Lint(stack *memory.ResolvedStack) []Finding {
	var findings []Finding
	for _, s := range stack.Sources {
		findings = append(findings, lintFileTooLong(s)...)
		findings = append(findings, lintVagueWords(s)...)
		findings = append(findings, lintDeadImports(s)...)
	}
	findings = append(findings, lintDuplicateLines(stack)...)
	return findings
}

func lintFileTooLong(s memory.Source) []Finding {
	n := countLines(s.Raw)
	if n <= maxLines {
		return nil
	}
	return []Finding{{
		File:     s.Path,
		Rule:     "file-too-long",
		Severity: SeverityWarn,
		Message:  fmt.Sprintf("%d lines (over the %d-line guideline); long files reduce adherence — consider splitting into path-scoped rules", n, maxLines),
	}}
}

func lintVagueWords(s memory.Source) []Finding {
	var out []Finding
	for i, line := range strings.Split(s.Raw, "\n") {
		lower := strings.ToLower(line)
		for _, w := range vagueWords {
			if containsWord(lower, w) {
				out = append(out, Finding{
					File:     s.Path,
					Line:     i + 1,
					Rule:     "vague-instruction",
					Severity: SeverityInfo,
					Message:  fmt.Sprintf("vague word %q — prefer a concrete, verifiable instruction", w),
					Snippet:  strings.TrimSpace(line),
				})
				break // one finding per line is enough
			}
		}
	}
	return out
}

func lintDeadImports(s memory.Source) []Finding {
	var out []Finding
	for _, imp := range s.Imports {
		if imp.Missing {
			out = append(out, Finding{
				File:     s.Path,
				Rule:     "dead-import",
				Severity: SeverityWarn,
				Message:  fmt.Sprintf("import %q does not resolve to a file", imp.Spec),
				Snippet:  imp.Spec,
			})
		}
	}
	return out
}

// lintDuplicateLines flags substantive lines that appear in more than one
// source file — these are contradiction candidates (the same topic stated in
// two places, possibly inconsistently). Trivial lines (headings, blanks, very
// short lines) are ignored to keep the signal high.
type lineLoc struct {
	file string
	line int
}

func lintDuplicateLines(stack *memory.ResolvedStack) []Finding {
	seen := map[string][]lineLoc{}
	for _, s := range stack.Sources {
		for i, line := range strings.Split(s.Raw, "\n") {
			norm := normalizeLine(line)
			if !isSubstantive(norm) {
				continue
			}
			seen[norm] = append(seen[norm], lineLoc{s.Path, i + 1})
		}
	}

	var out []Finding
	for norm, locs := range seen {
		files := distinctFiles(locs)
		if len(files) < 2 {
			continue
		}
		for _, l := range locs {
			out = append(out, Finding{
				File:     l.file,
				Line:     l.line,
				Rule:     "duplicate-line",
				Severity: SeverityInfo,
				Message:  fmt.Sprintf("this line also appears in %d other memory file(s); check for contradiction", len(files)-1),
				Snippet:  norm,
			})
		}
	}
	return out
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// normalizeLine lowercases, trims, and collapses whitespace so near-duplicates
// (differing only in spacing or case) are caught.
func normalizeLine(line string) string {
	return strings.ToLower(strings.Join(strings.Fields(line), " "))
}

// isSubstantive filters out lines too trivial to be meaningful duplicates.
func isSubstantive(norm string) bool {
	if len(norm) < 20 {
		return false
	}
	if strings.HasPrefix(norm, "#") {
		return false
	}
	return true
}

func distinctFiles(locs []lineLoc) map[string]bool {
	files := map[string]bool{}
	for _, l := range locs {
		files[l.file] = true
	}
	return files
}

// containsWord reports whether lowered text contains w as a whole word.
func containsWord(text, w string) bool {
	idx := 0
	for {
		i := strings.Index(text[idx:], w)
		if i < 0 {
			return false
		}
		start := idx + i
		end := start + len(w)
		leftOK := start == 0 || !isWordByte(text[start-1])
		rightOK := end == len(text) || !isWordByte(text[end])
		if leftOK && rightOK {
			return true
		}
		idx = end
	}
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
