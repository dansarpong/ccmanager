package sessions

import (
	"encoding/json"
	"testing"
)

func TestDecodeLineUnknownFields(t *testing.T) {
	// Extra/unknown fields must be tolerated, known fields still populated.
	raw := []byte(`{"type":"user","sessionId":"s1","cwd":"/p","brandNewField":42,"nested":{"x":1},"isMeta":false}`)
	l, ok := decodeLine(raw)
	if !ok {
		t.Fatal("decodeLine returned ok=false on a valid line with unknown fields")
	}
	if l.Type != "user" || l.SessionID != "s1" || l.CWD != "/p" {
		t.Errorf("known fields not populated: %+v", l)
	}
}

func TestDecodeLineMissingFields(t *testing.T) {
	// A line missing most fields decodes to zero values, not an error.
	l, ok := decodeLine([]byte(`{"type":"system"}`))
	if !ok {
		t.Fatal("decodeLine returned ok=false on a sparse line")
	}
	if l.Type != "system" {
		t.Errorf("type = %q, want system", l.Type)
	}
	if l.SessionID != "" || l.CWD != "" || l.Message != nil {
		t.Errorf("missing fields should be zero values: %+v", l)
	}
}

func TestDecodeLineBadJSONSkipped(t *testing.T) {
	for _, bad := range []string{``, `   `, `{"type":"user"`, `not json at all`, `{trailing`} {
		if _, ok := decodeLine([]byte(bad)); ok {
			t.Errorf("expected ok=false for %q", bad)
		}
	}
}

func TestRoleAndTextContentString(t *testing.T) {
	raw := json.RawMessage(`{"role":"user","content":"hello world"}`)
	role, text := roleAndText(raw)
	if role != "user" || text != "hello world" {
		t.Errorf("got role=%q text=%q", role, text)
	}
}

func TestRoleAndTextContentBlockList(t *testing.T) {
	raw := json.RawMessage(`{"role":"assistant","content":[
		{"type":"text","text":"first"},
		{"type":"tool_use","name":"Bash","input":{}},
		{"type":"text","text":"second"}
	]}`)
	role, text := roleAndText(raw)
	if role != "assistant" {
		t.Errorf("role = %q", role)
	}
	if want := "first\n[tool: Bash]\nsecond"; text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
}

func TestRoleAndTextMessageAsJSONString(t *testing.T) {
	// Some assistant lines store message as a JSON string that must be re-parsed.
	inner := `{"role":"assistant","content":[{"type":"text","text":"hi"}]}`
	b, _ := json.Marshal(inner) // produces a quoted JSON string
	role, text := roleAndText(json.RawMessage(b))
	if role != "assistant" || text != "hi" {
		t.Errorf("got role=%q text=%q", role, text)
	}
}

func TestRoleAndTextToolResultBlocks(t *testing.T) {
	raw := json.RawMessage(`{"role":"user","content":[
		{"type":"tool_result","content":[{"type":"text","text":"result text"}]}
	]}`)
	_, text := roleAndText(raw)
	if text != "result text" {
		t.Errorf("tool_result text = %q", text)
	}
}

func TestRoleAndTextEmpty(t *testing.T) {
	if role, text := roleAndText(nil); role != "" || text != "" {
		t.Errorf("nil message should yield empty, got role=%q text=%q", role, text)
	}
}

func TestMessageContentToolOnly(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"tool result only", `{"role":"user","content":[{"type":"tool_result","content":"ok"}]}`, true},
		{"tool use only", `{"role":"assistant","content":[{"type":"tool_use","name":"Read"}]}`, true},
		{"empty text beside tool use", `{"role":"assistant","content":[{"type":"text","text":""},{"type":"tool_use","name":"Bash"}]}`, true},
		{"prose beside tool use", `{"role":"assistant","content":[{"type":"text","text":"Reading it"},{"type":"tool_use","name":"Read"}]}`, false},
		{"content as string", `{"role":"user","content":"hello"}`, false},
		{"text blocks only", `{"role":"user","content":[{"type":"text","text":"hi"}]}`, false},
		{"message as JSON string", `"{\"role\":\"user\",\"content\":[{\"type\":\"tool_result\",\"content\":\"ok\"}]}"`, true},
	}
	for _, c := range cases {
		if _, _, got := messageContent(json.RawMessage(c.raw)); got != c.want {
			t.Errorf("%s: toolOnly = %v, want %v", c.name, got, c.want)
		}
	}
}
