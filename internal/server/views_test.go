package server

import (
	"testing"
	"time"

	"github.com/dansarpong/ccmanager/internal/store"
)

func TestMarkMatchesEscapesAndMarks(t *testing.T) {
	cases := []struct {
		label, q, want string
	}{
		{"Fix the indexer", "", "Fix the indexer"},
		{"Fix the indexer", "INDEX", "Fix the <mark>index</mark>er"},
		{"Fix the indexer", "fix index", "<mark>Fix</mark> the <mark>index</mark>er"},
		{"<script>alert(1)</script>", "script", "&lt;<mark>script</mark>&gt;alert(1)&lt;/<mark>script</mark>&gt;"},
		{"a & b", "amp", "a &amp; b"}, // never matches inside an entity
		{"cost (USD)", "(usd)", "cost <mark>(USD)</mark>"},
	}
	for _, c := range cases {
		if got := string(markMatches(c.label, c.q)); got != c.want {
			t.Errorf("markMatches(%q, %q) = %q, want %q", c.label, c.q, got, c.want)
		}
	}
}

func TestNewSessionViewStatus(t *testing.T) {
	now := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	cases := []struct {
		mtime  time.Time
		status string
	}{
		{now.Add(-time.Minute), "active"},
		{now.Add(-sessionActiveWindow + time.Second), "active"},
		{now.Add(-sessionActiveWindow), "idle"},
		{now.Add(-48 * time.Hour), "idle"},
	}
	for _, c := range cases {
		v := newSessionView(store.Session{FileMtime: c.mtime.Unix()}, now)
		if v.Status != c.status {
			t.Errorf("mtime %s before now: status %q, want %q", now.Sub(c.mtime), v.Status, c.status)
		}
	}
	if v := newSessionView(store.Session{}, now); v.Status != "idle" || v.StatusLabel != "Idle" {
		t.Errorf("zero mtime: got %q/%q, want idle/Idle", v.Status, v.StatusLabel)
	}
}

func TestNewMemoryEntryReadsFrontmatter(t *testing.T) {
	topic := "---\nname: simple-commit-messages\ndescription: \"One summary line only\"\nmetadata:\n  originSessionId: cf1a30d8-24ff-4a9b-b563-3c06315126af\n---\n\nKeep it short.\n"
	e := newMemoryEntry(InventoryItem{Path: "/p/memory/simple.md", Kind: "auto-memory-topic", Content: topic, ReadOnly: true})
	if e.Title != "simple-commit-messages" || e.Preview != "One summary line only" {
		t.Errorf("title/preview = %q/%q", e.Title, e.Preview)
	}
	if e.SourceSessionID != "cf1a30d8-24ff-4a9b-b563-3c06315126af" {
		t.Errorf("source session = %q", e.SourceSessionID)
	}
	if e.ID != memoryID("/p/memory/simple.md") || e.Body.ID != "body-"+e.ID {
		t.Errorf("ids = %q / %q", e.ID, e.Body.ID)
	}

	rule := "<!-- note -->\n# Working style\n\nDefault to no comment.\n"
	e = newMemoryEntry(InventoryItem{Path: "/c/rules/working-style.md", Kind: "user-rule", Content: rule})
	if e.Title != "Working style" || e.Preview != "Default to no comment." || e.SourceSessionID != "" {
		t.Errorf("rule entry = %q / %q / %q", e.Title, e.Preview, e.SourceSessionID)
	}

	e = newMemoryEntry(InventoryItem{Path: "/c/CLAUDE.md", Kind: "user", Content: ""})
	if e.Title != "CLAUDE.md" || e.KindLabel != "User CLAUDE.md" {
		t.Errorf("empty file entry = %q / %q", e.Title, e.KindLabel)
	}
}

func TestMatchesInventoryKind(t *testing.T) {
	cases := []struct {
		kind, filter string
		want         bool
	}{
		{"user", "", true},
		{"user", "user", true},
		{"user-rule", "user", false},
		{"auto-memory", "auto-memory", true},
		{"auto-memory-topic", "auto-memory", true},
		{"user-rule", "auto-memory", false},
	}
	for _, c := range cases {
		if got := matchesInventoryKind(c.kind, c.filter); got != c.want {
			t.Errorf("matchesInventoryKind(%q, %q) = %v", c.kind, c.filter, got)
		}
	}
}
