package sessions

import (
	"fmt"
	"os"

	"ccmanager/internal/store"
)

// TranscriptMessage is one rendered message for the transcript viewer.
type TranscriptMessage struct {
	Index     int    // 1-based position among the transcript's text messages
	Role      string // "user", "assistant", or "tool" for tool calls and results
	Timestamp string
	Text      string
}

// TranscriptPage is a slice of a transcript's messages.
type TranscriptPage struct {
	Messages []TranscriptMessage
	Offset   int
	Limit    int
	Total    int
	HasMore  bool
}

// ReadTranscript streams a transcript file and returns one page of its
// text-bearing messages. Message bodies are not kept in the index, so they are
// read from disk on demand. Streaming keeps memory bounded for large files.
func ReadTranscript(path string, offset, limit int) (TranscriptPage, error) {
	f, err := os.Open(path)
	if err != nil {
		return TranscriptPage{}, fmt.Errorf("opening transcript %s: %w", path, err)
	}
	defer f.Close()

	if limit <= 0 {
		limit = 50
	}
	page := TranscriptPage{Offset: offset, Limit: limit}
	idx := 0
	err = scanLines(f, func(b []byte) {
		l, ok := decodeLine(b)
		if !ok {
			return
		}
		if l.Type != "user" && l.Type != "assistant" {
			return
		}
		role, text, toolOnly := messageContent(l.Message)
		if text == "" {
			return
		}
		page.Total++
		if idx >= offset && len(page.Messages) < limit {
			role = orRole(role, l.Type)
			if toolOnly {
				role = "tool"
			}
			page.Messages = append(page.Messages, TranscriptMessage{
				Index:     idx + 1,
				Role:      role,
				Timestamp: l.Timestamp,
				Text:      text,
			})
		}
		idx++
	})
	if err != nil {
		return TranscriptPage{}, fmt.Errorf("reading transcript %s: %w", path, err)
	}
	page.HasMore = offset+limit < page.Total
	return page, nil
}

func orRole(role, typ string) string {
	if role != "" {
		return role
	}
	return typ
}

// ResumeChain describes a session's parent and children in the
// resume/branch graph.
type ResumeChain struct {
	Parent   *store.Session
	Children []store.Session
}
