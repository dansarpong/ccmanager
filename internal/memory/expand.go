package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// expandSource fills in s.Expanded (imports inlined, block comments stripped)
// and s.Imports (the imports found directly in this file, at depth 1). The
// recursive expansion follows @path imports up to MaxImportDepth hops,
// resolving each relative to the importing file, detecting cycles, and marking
// imports that fall outside the project subtree as external.
func expandSource(s *Source, projectRoot string) {
	if s.Raw == "" {
		s.Expanded = stripBlockComments(s.Raw)
		return
	}
	// chain holds the absolute paths currently being expanded, to detect cycles.
	chain := map[string]bool{s.Path: true}
	expanded, imports := expandText(s.Raw, s.Path, projectRoot, 1, chain)
	s.Expanded = stripBlockComments(expanded)
	s.Imports = imports
}

// expandText scans text (which came from fromFile) for @path imports outside
// code spans/fences, inlines each resolved import's expanded content, and
// returns the result plus the direct imports found at this level. depth is the
// hop count of the imports found here (1 at the top level).
func expandText(text, fromFile, projectRoot string, depth int, chain map[string]bool) (string, []Import) {
	var out strings.Builder
	var imports []Import

	inFence := false
	var fenceMarker string

	for _, line := range splitKeepNewline(text) {
		body, nl := line.body, line.newline

		if marker, ok := fenceToggle(body); ok {
			if !inFence {
				inFence, fenceMarker = true, marker
			} else if marker == fenceMarker {
				inFence = false
			}
			out.WriteString(body)
			out.WriteString(nl)
			continue
		}
		if inFence {
			out.WriteString(body)
			out.WriteString(nl)
			continue
		}

		spec, before, after, found := findImport(body)
		if !found {
			out.WriteString(body)
			out.WriteString(nl)
			continue
		}

		imp := resolveImport(spec, fromFile, projectRoot, depth, chain)
		imports = append(imports, imp)

		out.WriteString(before)
		if imp.ResolvedPath != "" && !imp.Missing && !imp.Circular && depth <= MaxImportDepth {
			out.WriteString("\n")
			out.WriteString(inlineImport(imp, projectRoot, depth, chain))
			out.WriteString("\n")
		} else {
			// Leave a visible marker so the rendering shows the unresolved import.
			out.WriteString(importMarker(imp))
		}
		out.WriteString(after)
		out.WriteString(nl)
	}
	return out.String(), imports
}

// inlineImport reads and recursively expands an imported file's content.
func inlineImport(imp Import, projectRoot string, depth int, chain map[string]bool) string {
	data, err := os.ReadFile(imp.ResolvedPath)
	if err != nil {
		return importMarker(imp)
	}
	chain[imp.ResolvedPath] = true
	defer delete(chain, imp.ResolvedPath)
	expanded, _ := expandText(string(data), imp.ResolvedPath, projectRoot, depth+1, chain)
	return expanded
}

// resolveImport turns a raw @spec into an Import, resolving the path relative to
// the importing file and flagging external/missing/circular.
func resolveImport(spec, fromFile, projectRoot string, depth int, chain map[string]bool) Import {
	imp := Import{Spec: spec, Depth: depth}
	raw := strings.TrimPrefix(spec, "@")

	var resolved string
	switch {
	case strings.HasPrefix(raw, "~/") || raw == "~":
		resolved = expandHome(raw)
	case filepath.IsAbs(raw):
		resolved = raw
	default:
		resolved = filepath.Join(filepath.Dir(fromFile), raw)
	}
	resolved = filepath.Clean(resolved)
	imp.ResolvedPath = resolved

	imp.External = isExternal(projectRoot, resolved)
	if chain[resolved] {
		imp.Circular = true
	}
	if !fileExists(resolved) {
		imp.Missing = true
	}
	return imp
}

func importMarker(imp Import) string {
	switch {
	case imp.Circular:
		return "[import skipped (circular): " + imp.Spec + "]"
	case imp.Missing:
		return "[import unresolved (missing): " + imp.Spec + "]"
	case imp.Depth > MaxImportDepth:
		return "[import skipped (max depth): " + imp.Spec + "]"
	default:
		return "[import: " + imp.Spec + "]"
	}
}

// findImport finds the first @path token in a line that is not inside a
// backtick code span, returning the spec and the text before/after it. A
// backticked `@path` is literal and must not be treated as an import.
func findImport(line string) (spec, before, after string, found bool) {
	inSpan := false
	i := 0
	for i < len(line) {
		c := line[i]
		if c == '`' {
			inSpan = !inSpan
			i++
			continue
		}
		if c == '@' && !inSpan && isImportStart(line, i) {
			end := importEnd(line, i)
			return line[i:end], line[:i], line[end:], true
		}
		i++
	}
	return "", "", "", false
}

// isImportStart reports whether the @ at index i begins an import: it must be
// at the start of the line or preceded by whitespace, and followed by a path
// character (so an email-like "a@b" is not treated as an import).
func isImportStart(line string, i int) bool {
	if i > 0 {
		prev := line[i-1]
		if prev != ' ' && prev != '\t' {
			return false
		}
	}
	if i+1 >= len(line) {
		return false
	}
	return isPathChar(line[i+1])
}

func importEnd(line string, start int) int {
	i := start + 1 // skip '@'
	for i < len(line) && isPathChar(line[i]) {
		i++
	}
	return i
}

func isPathChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '/', '.', '_', '-', '~':
		return true
	}
	return false
}

// fenceToggle reports whether a line opens or closes a fenced code block,
// returning the fence marker (``` or ~~~) so only a matching marker closes it.
func fenceToggle(body string) (string, bool) {
	t := strings.TrimSpace(body)
	if strings.HasPrefix(t, "```") {
		return "```", true
	}
	if strings.HasPrefix(t, "~~~") {
		return "~~~", true
	}
	return "", false
}

type lineParts struct {
	body    string
	newline string
}

// splitKeepNewline splits text into lines, preserving each line's trailing
// newline so reconstruction is faithful.
func splitKeepNewline(text string) []lineParts {
	var out []lineParts
	for len(text) > 0 {
		idx := strings.IndexByte(text, '\n')
		if idx < 0 {
			out = append(out, lineParts{body: text})
			break
		}
		out = append(out, lineParts{body: text[:idx], newline: "\n"})
		text = text[idx+1:]
	}
	return out
}
