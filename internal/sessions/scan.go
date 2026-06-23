package sessions

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"ccmanager/internal/store"
)

// ScanFile reads one transcript file and returns its session summary plus the
// text-bearing messages (for the FTS index). It streams line by line and never
// holds the whole file in memory; the largest transcripts on disk are ~30 MB.
//
// A truncated or malformed final line (Claude Code may be mid-write) is
// skipped, not treated as an error.
func ScanFile(path string) (store.Session, []store.Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return store.Session{}, nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return store.Session{}, nil, fmt.Errorf("stat %s: %w", path, err)
	}

	se := store.Session{
		FilePath:  path,
		FileSize:  info.Size(),
		FileMtime: info.ModTime().Unix(),
	}
	var msgs []store.Message
	seq := 0
	firstParentSet := false

	err = scanLines(f, func(b []byte) {
		l, ok := decodeLine(b)
		if !ok {
			return
		}
		if l.SessionID != "" {
			se.SessionID = l.SessionID
		}
		if l.CWD != "" {
			se.ProjectPath = l.CWD
		}
		if l.Timestamp != "" {
			se.LastActivity = l.Timestamp
		}

		// Titles are last-write-wins envelopes keyed by sessionId.
		switch l.Type {
		case "custom-title":
			if l.CustomTitle != "" {
				se.CustomTitle = l.CustomTitle
			}
		case "ai-title":
			if l.AITitle != "" {
				se.AITitle = l.AITitle
			}
		}

		if l.Type != "user" && l.Type != "assistant" {
			return
		}

		// The first message line's parentUuid links a resumed/branched
		// session to its origin.
		if !firstParentSet {
			se.FirstParentUUID = l.ParentUUID
			firstParentSet = true
		}

		role, text := roleAndText(l.Message)
		se.MessageCount++

		// First "real" (non-meta) user prompt becomes the fallback name.
		if l.Type == "user" && !l.IsMeta && se.FirstPrompt == "" {
			if t := strings.TrimSpace(text); t != "" {
				se.FirstPrompt = t
			}
		}

		seq++
		msgs = append(msgs, store.Message{
			SessionID:  l.SessionID,
			UUID:       l.UUID,
			ParentUUID: l.ParentUUID,
			Role:       role,
			Timestamp:  l.Timestamp,
			Seq:        seq,
			Body:       text,
		})
	})
	if err != nil {
		return store.Session{}, nil, fmt.Errorf("scanning %s: %w", path, err)
	}

	se.DisplayName = DisplayName(se)
	return se, msgs, nil
}

// scanLines calls fn once per line. It uses bufio.Reader.ReadString rather
// than bufio.Scanner so there is no per-line size cap: individual transcript
// lines (large tool results, pasted content) can be many megabytes. A final
// line without a trailing newline is still delivered, which is also how a
// truncated final line is tolerated.
func scanLines(r io.Reader, fn func([]byte)) error {
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		s, err := br.ReadString('\n')
		if len(s) > 0 {
			fn([]byte(strings.TrimRight(s, "\n")))
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading lines: %w", err)
		}
	}
}
