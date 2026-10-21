// Package sessions reads Claude Code transcript files (.jsonl) defensively
// and turns them into indexable session summaries and messages.
//
// The on-disk JSONL format is not a stable contract and varies between Claude
// Code versions. Everything this package knows about the format is encoded
// here and in summary.go, so there is one place to update when the format
// shifts. Verified against Claude Code v2.1.186 (see DISCOVERY in the plan).
package sessions

import (
	"encoding/json"
	"strings"
)

// line is the subset of fields ccmanager cares about. Unknown fields are
// ignored by encoding/json, and missing fields decode to their zero value, so
// this struct tolerates format drift. The message field is polymorphic
// (object on user lines, sometimes a JSON string on assistant lines) so it is
// kept raw and decoded separately.
type line struct {
	Type        string          `json:"type"`
	UUID        string          `json:"uuid"`
	ParentUUID  string          `json:"parentUuid"`
	SessionID   string          `json:"sessionId"`
	CWD         string          `json:"cwd"`
	Timestamp   string          `json:"timestamp"`
	IsMeta      bool            `json:"isMeta"`
	Message     json.RawMessage `json:"message"`
	CustomTitle string          `json:"customTitle"`
	AITitle     string          `json:"aiTitle"`
}

// decodeLine parses a single JSONL line. It returns ok=false for blank lines
// and lines that are not valid JSON (e.g. a truncated final line mid-write),
// so a bad line never crashes a scan.
func decodeLine(b []byte) (line, bool) {
	if len(strings.TrimSpace(string(b))) == 0 {
		return line{}, false
	}
	var l line
	if err := json.Unmarshal(b, &l); err != nil {
		return line{}, false
	}
	return l, true
}

// roleAndText extracts the role and plain text from a polymorphic message
// field. It handles message-as-object and message-as-string, and within an
// object, content-as-string and content-as-block-list (concatenating the text
// of "text" blocks). Anything it cannot interpret yields empty strings.
func roleAndText(raw json.RawMessage) (role, text string) {
	role, text, _ = messageContent(raw)
	return role, text
}

// messageContent is roleAndText plus whether the message carries only tool
// calls or tool results and no prose of its own.
func messageContent(raw json.RawMessage) (role, text string, toolOnly bool) {
	if len(raw) == 0 {
		return "", "", false
	}

	// message may itself be a JSON string holding the real object.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if strings.TrimSpace(s) == "" {
				return "", "", false
			}
			// Re-parse the inner string if it looks like JSON, else treat as text.
			if trimmed := strings.TrimSpace(s); strings.HasPrefix(trimmed, "{") {
				return messageContent(json.RawMessage(s))
			}
			return "", s, false
		}
		return "", "", false
	}

	var m struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", "", false
	}
	return m.Role, contentText(m.Content), onlyToolBlocks(m.Content)
}

// onlyToolBlocks reports whether a content block list holds at least one
// tool_use or tool_result block and no non-empty text block.
func onlyToolBlocks(raw json.RawMessage) bool {
	if len(raw) == 0 || raw[0] != '[' {
		return false
	}
	var blocks []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return false
	}
	tool := false
	for _, b := range blocks {
		switch jsonString(b["type"]) {
		case "tool_use", "tool_result":
			tool = true
		case "text":
			if jsonString(b["text"]) != "" {
				return false
			}
		}
	}
	return tool
}

// contentText flattens a content field to plain text. Content is either a
// string or a list of blocks; for the list we keep the text of "text" blocks
// and summarize tool calls/results compactly so they are still searchable.
func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return ""
	}

	var blocks []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		bt := jsonString(b["type"])
		switch bt {
		case "text":
			if t := jsonString(b["text"]); t != "" {
				parts = append(parts, t)
			}
		case "tool_use":
			if name := jsonString(b["name"]); name != "" {
				parts = append(parts, "[tool: "+name+"]")
			}
		case "tool_result":
			if t := toolResultText(b["content"]); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// toolResultText pulls text out of a tool_result content field, which may be a
// string or a list of text blocks.
func toolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return ""
	}
	var blocks []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if jsonString(b["type"]) == "text" {
			if t := jsonString(b["text"]); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// jsonString unmarshals a raw value as a string, returning "" on any mismatch.
func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}
