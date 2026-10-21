package server

import (
	"testing"
	"time"
)

func TestShortID(t *testing.T) {
	cases := map[string]string{
		"":                                     "",
		"a3f9":                                 "a3f9",
		"a3f9c2e1":                             "a3f9c2e1",
		"a3f9c2e1-5b7d-4c1e-9f02-8e6a1d3b7c45": "a3f9c2e1",
	}
	for in, want := range cases {
		if got := shortID(in); got != want {
			t.Errorf("shortID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCompactNum(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{214, "214"},
		{999, "999"},
		{1000, "1k"},
		{1050, "1.1k"},
		{48213, "48.2k"},
		{999_949, "999.9k"},
		{999_950, "1M"},
		{1_250_000, "1.3M"},
		{2_000_000_000, "2B"},
		{-48213, "-48.2k"},
	}
	for _, c := range cases {
		if got := compactNum(c.in); got != c.want {
			t.Errorf("compactNum(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRelTimeAt(t *testing.T) {
	now := time.Date(2026, 10, 21, 14, 5, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want string
	}{
		{"", "—"},
		{"not a time", "not a time"},
		{"2026-10-21T14:04:30Z", "just now"},
		{"2026-10-21T14:05:20Z", "just now"}, // slight clock skew
		{"2026-10-21T14:03:00Z", "2m ago"},
		{"2026-10-21T14:03:00.123456Z", "1m ago"}, // 1m59.9s floors
		{"2026-10-21T11:05:00Z", "3h ago"},
		{"2026-10-20T09:00:00Z", "yesterday"},
		{"2026-10-19T23:00:00Z", "Oct 19"},
		{"2026-10-12T08:00:00Z", "Oct 12"},
		{"2025-10-12T08:00:00Z", "Oct 12, 2025"},
		{"2026-11-20T14:05:00Z", "Nov 20"}, // future, e.g. a purge date
		{"2027-01-02T00:00:00Z", "Jan 2, 2027"},
		{"2026-10-21T16:05:00+02:00", "just now"}, // offsets convert before comparing
	}
	for _, c := range cases {
		if got := relTimeAt(c.in, now); got != c.want {
			t.Errorf("relTimeAt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClockIn(t *testing.T) {
	accra := time.FixedZone("GMT+2", 2*60*60)
	cases := []struct {
		in   string
		loc  *time.Location
		want string
	}{
		{"", time.UTC, ""},
		{"garbage", time.UTC, "garbage"},
		{"2026-10-21T14:02:11Z", time.UTC, "14:02:11"},
		{"2026-10-21T14:02:11.987Z", time.UTC, "14:02:11"},
		{"2026-10-21T14:02:11Z", accra, "16:02:11"},
	}
	for _, c := range cases {
		if got := clockIn(c.in, c.loc); got != c.want {
			t.Errorf("clockIn(%q, %s) = %q, want %q", c.in, c.loc, got, c.want)
		}
	}
}

func TestCodeBlockNumbersLines(t *testing.T) {
	v := codeBlock("b1", "~/.claude/CLAUDE.md", "# Rules\n\n- one\n")
	if v.ID != "b1" || v.Diff {
		t.Fatalf("unexpected block header: %+v", v)
	}
	want := []codeLine{{N: 1, Text: "# Rules"}, {N: 2, Text: ""}, {N: 3, Text: "- one"}}
	if len(v.Lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(v.Lines), len(want))
	}
	for i, w := range want {
		if v.Lines[i] != w {
			t.Errorf("line %d = %+v, want %+v", i, v.Lines[i], w)
		}
	}
}

func TestDiffBlockReadsAdvisorDiff(t *testing.T) {
	diff := "  keep\n- old line\n+ new line\n+ \n  tail\n"
	v := diffBlock("d1", "f.md", diff)
	if !v.Diff || v.Added != 2 || v.Removed != 1 {
		t.Fatalf("Diff=%v Added=%d Removed=%d; want true, 2, 1", v.Diff, v.Added, v.Removed)
	}
	want := []codeLine{
		{Text: "keep"},
		{Kind: "del", Text: "old line"},
		{Kind: "add", Text: "new line"},
		{Kind: "add", Text: ""},
		{Text: "tail"},
	}
	if len(v.Lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(v.Lines), len(want))
	}
	for i, w := range want {
		if v.Lines[i] != w {
			t.Errorf("line %d = %+v, want %+v", i, v.Lines[i], w)
		}
	}
}
