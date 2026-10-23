package sessions

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dansarpong/ccmanager/internal/config"
	"github.com/dansarpong/ccmanager/internal/store"
)

// IndexResult reports what an Index run did.
type IndexResult struct {
	Scanned int // files re-scanned (new or changed)
	Skipped int // files unchanged since last index
	Failed  int // files that errored (skipped, not fatal)
	Pruned  int // indexed sessions whose transcript file no longer exists
}

// Index walks ProjectsDir/<project>/*.jsonl and (re-)indexes each file whose
// mtime or size differs from what is stored. The first run indexes everything;
// a refresh only re-scans changed files. A single unreadable or malformed file
// is counted as failed and skipped, never aborting the whole run.
//
// Rows for transcripts that have disappeared from disk (e.g. deleted outside
// ccmanager) are pruned, so the list never shows a session that 404s/500s when
// opened.
func Index(st *store.Store, cfg config.Config) (IndexResult, error) {
	var res IndexResult

	known, err := st.IndexedFiles()
	if err != nil {
		return res, fmt.Errorf("loading indexed files: %w", err)
	}

	entries, err := os.ReadDir(cfg.ProjectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return res, fmt.Errorf("reading projects dir: %w", err)
	}

	seen := make(map[string]bool, len(known))
	for _, projEntry := range entries {
		if !projEntry.IsDir() {
			continue
		}
		projDir := projEntry.Name()
		matches, err := filepath.Glob(filepath.Join(cfg.ProjectsDir, projDir, "*.jsonl"))
		if err != nil {
			return res, fmt.Errorf("globbing transcripts in %s: %w", projDir, err)
		}
		for _, path := range matches {
			info, err := os.Stat(path)
			if err != nil {
				res.Failed++
				continue
			}
			seen[path] = true
			if prev, ok := known[path]; ok && prev.Mtime == info.ModTime().Unix() && prev.Size == info.Size() {
				res.Skipped++
				continue
			}

			se, msgs, err := ScanFile(path)
			if err != nil || se.SessionID == "" {
				res.Failed++
				continue
			}
			se.ProjectDir = projDir
			se.IndexedAt = time.Now().UTC().Format(time.RFC3339)

			if err := st.UpsertSession(se); err != nil {
				return res, fmt.Errorf("indexing %s: %w", path, err)
			}
			if err := st.ReplaceMessages(se.SessionID, msgs); err != nil {
				return res, fmt.Errorf("indexing messages for %s: %w", path, err)
			}
			res.Scanned++
		}
	}

	// Prune any indexed session whose file was not seen on this walk. A stat
	// error above leaves the file out of `seen`, so a transiently unreadable
	// file would be pruned and re-indexed next run — acceptable, since the row
	// would otherwise be unusable anyway.
	for path, meta := range known {
		if seen[path] {
			continue
		}
		if err := st.DeleteSessionRowsByFile(path, meta.SessionID); err != nil {
			return res, fmt.Errorf("pruning missing transcript %s: %w", path, err)
		}
		res.Pruned++
	}
	return res, nil
}
