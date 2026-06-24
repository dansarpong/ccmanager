package advisor

import (
	"strings"
	"testing"
)

func TestApplyRewriteReplaces(t *testing.T) {
	current := "# Memory\nPlease format the code properly.\nRun tests.\n"
	sug := RewriteSuggestion{
		File:       "/p/CLAUDE.md",
		Original:   "Please format the code properly.",
		Suggestion: "Use 2-space indentation; run gofmt before committing.",
	}
	edit := ApplyRewrite(current, sug)
	if !edit.Applied {
		t.Fatal("expected the original to be found and replaced")
	}
	if !strings.Contains(edit.NewContent, "2-space indentation") {
		t.Errorf("new content missing the suggestion: %q", edit.NewContent)
	}
	if strings.Contains(edit.NewContent, "format the code properly") {
		t.Errorf("original text should be gone: %q", edit.NewContent)
	}
	if !strings.Contains(edit.Diff, "- Please format the code properly.") {
		t.Errorf("diff should show the removed line:\n%s", edit.Diff)
	}
	if !strings.Contains(edit.Diff, "+ Use 2-space indentation") {
		t.Errorf("diff should show the added line:\n%s", edit.Diff)
	}
}

func TestApplyRewriteNotFound(t *testing.T) {
	current := "# Memory\nRun tests.\n"
	edit := ApplyRewrite(current, RewriteSuggestion{Original: "does not exist", Suggestion: "x"})
	if edit.Applied {
		t.Error("Applied should be false when original is not present")
	}
	if edit.NewContent != current {
		t.Error("content should be unchanged when original is not found")
	}
}

func TestExtractJSON(t *testing.T) {
	cases := map[string]string{
		`[{"a":1}]`:                          `[{"a":1}]`,
		"```json\n[{\"a\":1}]\n```":          `[{"a":1}]`,
		"Here are the results:\n[{\"a\":1}]": `[{"a":1}]`,
		"no json here":                       `[]`,
	}
	for in, want := range cases {
		if got := extractJSON(in); got != want {
			t.Errorf("extractJSON(%q) = %q, want %q", in, got, want)
		}
	}
}
