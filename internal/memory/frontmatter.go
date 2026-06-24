package memory

import "strings"

// parsePathsFrontmatter extracts the paths: glob list from a rule file's YAML
// frontmatter, if present. It does a small, dependency-free parse rather than
// pulling in a YAML library: frontmatter is a leading "---" block, and we only
// need the paths: sequence. Returns ok=false if there is no paths: field.
//
// Supported shapes:
//
//	---
//	paths:
//	  - "src/**/*.ts"
//	  - "lib/**/*.ts"
//	---
//
// and the inline form `paths: ["a", "b"]`.
func parsePathsFrontmatter(raw string) (paths []string, ok bool) {
	fm, rest := splitFrontmatter(raw)
	if !rest {
		return nil, false
	}
	lines := strings.Split(fm, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "paths:") {
			continue
		}
		ok = true
		inline := strings.TrimSpace(strings.TrimPrefix(line, "paths:"))
		if strings.HasPrefix(inline, "[") {
			return parseInlineList(inline), true
		}
		// Block form: subsequent "- value" lines, more indented.
		for j := i + 1; j < len(lines); j++ {
			item := strings.TrimSpace(lines[j])
			if !strings.HasPrefix(item, "-") {
				break
			}
			val := strings.TrimSpace(strings.TrimPrefix(item, "-"))
			if v := unquote(val); v != "" {
				paths = append(paths, v)
			}
		}
		return paths, true
	}
	return nil, false
}

// splitFrontmatter returns the YAML frontmatter body (between the leading "---"
// fences) and whether a frontmatter block was found.
func splitFrontmatter(raw string) (string, bool) {
	trimmed := strings.TrimPrefix(raw, "\uFEFF") // tolerate a leading UTF-8 BOM
	if !strings.HasPrefix(trimmed, "---") {
		return "", false
	}
	rest := trimmed[len("---"):]
	if !strings.HasPrefix(rest, "\n") && !strings.HasPrefix(rest, "\r\n") {
		return "", false
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func parseInlineList(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	var out []string
	for _, part := range strings.Split(s, ",") {
		if v := unquote(strings.TrimSpace(part)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
