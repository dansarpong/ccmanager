package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTranscript writes lines (joined by newlines) plus an optional truncated
// trailing fragment with NO newline, simulating a file Claude Code is mid-write.
func writeTranscript(t *testing.T, lines []string, truncatedTail string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	var content string
	for _, l := range lines {
		content += l + "\n"
	}
	content += truncatedTail // no trailing newline
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing transcript: %v", err)
	}
	return path
}

func TestScanFileTruncatedFinalLine(t *testing.T) {
	lines := []string{
		`{"type":"user","sessionId":"sX","cwd":"/work","timestamp":"2026-01-01T00:00:00Z","uuid":"u1","message":{"role":"user","content":"first real prompt"}}`,
		`{"type":"assistant","sessionId":"sX","uuid":"u2","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"reply"}]}}`,
	}
	// A final line that was cut off mid-write (invalid JSON, no newline).
	truncated := `{"type":"assistant","sessionId":"sX","message":{"role":"assi`

	path := writeTranscript(t, lines, truncated)
	se, msgs, err := ScanFile(path)
	if err != nil {
		t.Fatalf("ScanFile errored on a truncated final line: %v", err)
	}
	if se.SessionID != "sX" {
		t.Errorf("sessionID = %q, want sX", se.SessionID)
	}
	if se.ProjectPath != "/work" {
		t.Errorf("projectPath = %q, want /work", se.ProjectPath)
	}
	// Two complete messages; the truncated third is skipped.
	if se.MessageCount != 2 {
		t.Errorf("messageCount = %d, want 2 (truncated line skipped)", se.MessageCount)
	}
	if len(msgs) != 2 {
		t.Errorf("got %d messages, want 2", len(msgs))
	}
	if se.FirstPrompt != "first real prompt" {
		t.Errorf("firstPrompt = %q", se.FirstPrompt)
	}
}

func TestScanFileSkipsMetaForFirstPrompt(t *testing.T) {
	lines := []string{
		`{"type":"user","sessionId":"sM","isMeta":true,"message":{"role":"user","content":"<local-command-caveat>ignore me</local-command-caveat>"}}`,
		`{"type":"user","sessionId":"sM","message":{"role":"user","content":"the actual question"}}`,
	}
	path := writeTranscript(t, lines, "")
	se, _, err := ScanFile(path)
	if err != nil {
		t.Fatalf("ScanFile: %v", err)
	}
	if se.FirstPrompt != "the actual question" {
		t.Errorf("firstPrompt = %q, want the non-meta prompt", se.FirstPrompt)
	}
}

func TestScanFileTitlePrecedence(t *testing.T) {
	lines := []string{
		`{"type":"user","sessionId":"sT","message":{"role":"user","content":"prompt"}}`,
		`{"type":"ai-title","sessionId":"sT","aiTitle":"Auto Generated"}`,
		`{"type":"custom-title","sessionId":"sT","customTitle":"My Custom Title"}`,
	}
	path := writeTranscript(t, lines, "")
	se, _, _ := ScanFile(path)
	if got := DisplayName(se); got != "My Custom Title" {
		t.Errorf("DisplayName = %q, want custom title to win", got)
	}
}
