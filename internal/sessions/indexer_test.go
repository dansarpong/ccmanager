package sessions

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ccmanager/internal/config"
	"ccmanager/internal/store"
)

// writeProjectTranscript writes one transcript line into
// ProjectsDir/<projDir>/<sessionID>.jsonl and returns its path.
func writeProjectTranscript(t *testing.T, projectsDir, projDir, sessionID string) string {
	t.Helper()
	dir := filepath.Join(projectsDir, projDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, sessionID+".jsonl")
	line := `{"type":"user","sessionId":"` + sessionID + `","cwd":"/work","timestamp":"2026-01-01T00:00:00Z","uuid":"u1","message":{"role":"user","content":"hello"}}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatalf("writing transcript: %v", err)
	}
	return path
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// TestIndexPrunesMissingTranscripts is the regression test for a session that
// shows in the list but 500s when opened: when a transcript file is deleted
// outside ccmanager, re-indexing must drop its row (and message/FTS rows).
func TestIndexPrunesMissingTranscripts(t *testing.T) {
	projectsDir := t.TempDir()
	cfg := config.Config{ProjectsDir: projectsDir}
	st := newTestStore(t)

	keepPath := writeProjectTranscript(t, projectsDir, "-home-work", "sKeep")
	gonePath := writeProjectTranscript(t, projectsDir, "-home-work", "sGone")
	_ = keepPath

	res, err := Index(st, cfg)
	if err != nil {
		t.Fatalf("initial Index: %v", err)
	}
	if res.Scanned != 2 {
		t.Fatalf("initial scanned = %d, want 2", res.Scanned)
	}

	// Delete one transcript on disk, as if removed outside ccmanager.
	if err := os.Remove(gonePath); err != nil {
		t.Fatalf("removing transcript: %v", err)
	}

	res, err = Index(st, cfg)
	if err != nil {
		t.Fatalf("re-Index: %v", err)
	}
	if res.Pruned != 1 {
		t.Errorf("pruned = %d, want 1", res.Pruned)
	}

	// The surviving session is still there...
	if _, err := st.GetSession("sKeep"); err != nil {
		t.Errorf("kept session disappeared: %v", err)
	}
	// ...and the deleted one is gone from the index entirely.
	if _, err := st.GetSession("sGone"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetSession(sGone) err = %v, want sql.ErrNoRows", err)
	}
}
