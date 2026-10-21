package server

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"
)

// parseTemplates parses all embedded templates with shared helper functions.
func parseTemplates() (*template.Template, error) {
	funcs := template.FuncMap{
		"prettyTime": prettyTime,
		"humanSize":  humanSize,
		"add":        func(a, b int) int { return a + b },
		"shortID":    shortID,
		"compactNum": compactNum,
		"relTime":    relTime,
		"clock":      clock,
		"codeBlock":  codeBlock,
		"diffBlock":  diffBlock,
		"tildePath":  tildePath,
	}
	return template.New("").Funcs(funcs).ParseFS(templatesFS, "templates/*.html")
}

// prettyTime renders an ISO-8601 timestamp as a compact local time, or returns
// the raw value if it cannot be parsed.
func prettyTime(iso string) string {
	if iso == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return t.Local().Format("2006-01-02 15:04")
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KB", "MB", "GB", "TB"}[exp]
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), suffix)
}

// shortID returns the first 8 characters of an ID, the form shown in rows.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// compactNum renders a count for dense rows: 214 → "214", 48213 → "48.2k".
func compactNum(n int) string {
	if n < 0 {
		return "-" + compactNum(-n)
	}
	// Each bound is where one decimal would round up to the next unit
	// (999950 → "1000.0k"), so those values move up a unit instead.
	switch {
	case n < 1000:
		return strconv.Itoa(n)
	case n < 999_950:
		return oneDecimal(float64(n)/1e3) + "k"
	case n < 999_950_000:
		return oneDecimal(float64(n)/1e6) + "M"
	default:
		return oneDecimal(float64(n)/1e9) + "B"
	}
}

// oneDecimal rounds half away from zero (1.25 → "1.3"); FormatFloat alone
// rounds half to even.
func oneDecimal(f float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(math.Round(f*10)/10, 'f', 1, 64), ".0")
}

// relTime renders an RFC 3339 timestamp relative to now: "just now", "2m ago",
// "3h ago", "yesterday", "Oct 12", or "Oct 12, 2025" outside this year.
func relTime(iso string) string {
	return relTimeAt(iso, time.Now())
}

func relTimeAt(iso string, now time.Time) string {
	if iso == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	t = t.In(now.Location())
	d := now.Sub(t)
	switch {
	case d < -time.Minute:
		// Future dates, such as when trash purges, read best as a date.
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	default:
		y, m, day := now.AddDate(0, 0, -1).Date()
		if ty, tm, td := t.Date(); ty == y && tm == m && td == day {
			return "yesterday"
		}
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}

// clock renders an RFC 3339 timestamp as a local wall-clock time, "14:02:11".
func clock(iso string) string {
	return clockIn(iso, time.Local)
}

func clockIn(iso string, loc *time.Location) string {
	if iso == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return t.In(loc).Format("15:04:05")
}

// codeLine and codeBlockView are the data for the "code-block" partial.
type codeLine struct {
	Kind string // "", "add", "del" or "hunk"
	N    int    // line number to show; 0 shows none
	Text string
}

type codeBlockView struct {
	ID             string
	Path           string
	Diff           bool
	Added, Removed int
	Lines          []codeLine
}

// codeBlock numbers the lines of a file for the "code-block" partial.
func codeBlock(id, path, text string) codeBlockView {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	v := codeBlockView{ID: id, Path: path, Lines: make([]codeLine, len(lines))}
	for i, l := range lines {
		v.Lines[i] = codeLine{N: i + 1, Text: l}
	}
	return v
}

// diffBlock turns an advisor diff into "code-block" data. It reads the two
// character prefixes advisor.unifiedDiff writes ("+ ", "- ", "  "); that diff
// carries no line positions, so no numbers are shown.
func diffBlock(id, path, diff string) codeBlockView {
	v := codeBlockView{ID: id, Path: path, Diff: true}
	for _, l := range strings.Split(strings.TrimSuffix(diff, "\n"), "\n") {
		line := codeLine{Text: l}
		switch {
		case strings.HasPrefix(l, "+ "):
			line.Kind, line.Text = "add", l[2:]
			v.Added++
		case strings.HasPrefix(l, "- "):
			line.Kind, line.Text = "del", l[2:]
			v.Removed++
		case strings.HasPrefix(l, "  "):
			line.Text = l[2:]
		}
		v.Lines = append(v.Lines, line)
	}
	return v
}
