package safeio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.md")
	if err := AtomicWrite(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q", got)
	}
	// No temp files should be left behind.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file, found %d (temp left behind?)", len(entries))
	}
}

func TestChangeDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.md")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := Snapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Changed(path, snap)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("file should be unchanged immediately after snapshot")
	}

	// A different size is detected even if mtime resolution is coarse.
	if err := os.WriteFile(path, []byte("v2-longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err = Changed(path, snap)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("file should be detected as changed after rewrite")
	}
}

func TestChangedMissingFile(t *testing.T) {
	changed, err := Changed(filepath.Join(t.TempDir(), "nope"), FileState{Mtime: 1, Size: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("a missing file should be reported as changed")
	}
}
