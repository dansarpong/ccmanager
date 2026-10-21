package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTranscriptIndexAndToolRole(t *testing.T) {
	jsonl := `{"type":"user","timestamp":"2026-10-21T14:02:11Z","message":{"role":"user","content":"Fix the indexer"}}
{"type":"assistant","timestamp":"2026-10-21T14:02:15Z","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read"}]}}
{"type":"system","timestamp":"2026-10-21T14:02:16Z"}
{"type":"user","timestamp":"2026-10-21T14:02:16Z","message":{"role":"user","content":[{"type":"tool_result","content":"package sessions"}]}}
{"type":"assistant","timestamp":"2026-10-21T14:02:31Z","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}
`
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, []byte(jsonl), 0o644); err != nil {
		t.Fatal(err)
	}

	page, err := ReadTranscript(path, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || !page.HasMore {
		t.Fatalf("Total = %d, HasMore = %v; want 4, true", page.Total, page.HasMore)
	}
	want := []struct {
		index int
		role  string
	}{{2, "tool"}, {3, "tool"}}
	if len(page.Messages) != len(want) {
		t.Fatalf("got %d messages, want %d", len(page.Messages), len(want))
	}
	for i, w := range want {
		if m := page.Messages[i]; m.Index != w.index || m.Role != w.role {
			t.Errorf("message %d = {Index %d, Role %q}, want {%d, %q}", i, m.Index, m.Role, w.index, w.role)
		}
	}
}
