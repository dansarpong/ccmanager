package server

import (
	"fmt"
	"html/template"
	"time"
)

// parseTemplates parses all embedded templates with shared helper functions.
func parseTemplates() (*template.Template, error) {
	funcs := template.FuncMap{
		"prettyTime": prettyTime,
		"humanSize":  humanSize,
		"add":        func(a, b int) int { return a + b },
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
