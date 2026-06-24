package server

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InventoryItem is one memory file found on the machine.
type InventoryItem struct {
	Path     string
	Kind     string // "user" | "user-rule" | "auto-memory" | "auto-memory-topic"
	Lines    int
	Size     int64
	ReadOnly bool // auto-memory is view-and-delete only in v1
	TooLong  bool // over the 200-line guideline (a quick lint badge)
}

const inventoryLineWarn = 200

// scanInventory lists every memory file ccmanager knows how to find: the
// user-level CLAUDE.md and rules, and every project's auto-memory directory
// under ProjectsDir. Per-project CLAUDE.md files in the working trees are shown
// in the per-project resolved view (which knows the working dir); the global
// inventory focuses on the files living under the config root.
func scanInventory(configRoot, projectsDir string) ([]InventoryItem, error) {
	var items []InventoryItem

	addFile := func(path, kind string, readOnly bool) {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		data, _ := os.ReadFile(path)
		lines := countFileLines(data)
		items = append(items, InventoryItem{
			Path:     path,
			Kind:     kind,
			Lines:    lines,
			Size:     info.Size(),
			ReadOnly: readOnly,
			TooLong:  lines > inventoryLineWarn,
		})
	}

	if configRoot != "" {
		addFile(filepath.Join(configRoot, "CLAUDE.md"), "user", false)
		for _, r := range mdFiles(filepath.Join(configRoot, "rules")) {
			addFile(r, "user-rule", false)
		}
	}

	// Auto-memory under each project directory.
	entries, err := os.ReadDir(projectsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading projects dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		memDir := filepath.Join(projectsDir, e.Name(), "memory")
		for _, m := range mdFiles(memDir) {
			kind := "auto-memory-topic"
			if strings.EqualFold(filepath.Base(m), "MEMORY.md") {
				kind = "auto-memory"
			}
			addFile(m, kind, true) // auto-memory is read-only in v1
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items, nil
}

func mdFiles(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func countFileLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}
