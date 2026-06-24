// Package advisor inspects resolved memory and proposes improvements. The
// static lint (this file and lint.go) is always-on, offline, and instant. The
// LLM-backed analysis (added in a later stage) is propose-only and never
// writes to disk on its own.
package advisor

// Severity ranks a finding.
type Severity string

const (
	SeverityInfo Severity = "info"
	SeverityWarn Severity = "warn"
)

// Finding is one static-lint result, pointing back to a source file and line.
type Finding struct {
	File     string
	Line     int // 1-based; 0 means the finding is about the whole file
	Rule     string
	Severity Severity
	Message  string
	Snippet  string
}
