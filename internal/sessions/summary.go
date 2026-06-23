package sessions

import (
	"strings"

	"ccmanager/internal/store"
)

// DisplayName picks a human-readable name for a session. Precedence, verified
// against Claude Code v2.1.186: a user-set custom title, then an
// auto-generated AI title, then the first non-meta user prompt, then a
// placeholder. (There is no "summary" line type in this version.)
func DisplayName(s store.Session) string {
	if t := strings.TrimSpace(s.CustomTitle); t != "" {
		return t
	}
	if t := strings.TrimSpace(s.AITitle); t != "" {
		return t
	}
	if p := firstLine(s.FirstPrompt); p != "" {
		return p
	}
	return "(untitled)"
}

// firstLine returns the first non-empty line of s, trimmed and length-capped,
// suitable for a one-line label.
func firstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		const max = 120
		if len(ln) > max {
			return ln[:max] + "…"
		}
		return ln
	}
	return ""
}
