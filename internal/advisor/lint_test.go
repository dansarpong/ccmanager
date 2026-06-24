package advisor

import (
	"strings"
	"testing"

	"ccmanager/internal/memory"
)

func TestLintVagueAndDuplicateAndDead(t *testing.T) {
	stack := &memory.ResolvedStack{
		Sources: []memory.Source{
			{
				Path: "/a/CLAUDE.md",
				Raw:  "Format the code properly before committing changes here.\nAlways run the full test suite before you push to main.",
				Imports: []memory.Import{
					{Spec: "@missing.md", Missing: true},
				},
			},
			{
				Path: "/b/CLAUDE.md",
				Raw:  "Always run the full test suite before you push to main.",
			},
		},
	}

	findings := Lint(stack)

	got := map[string]int{}
	for _, f := range findings {
		got[f.Rule]++
	}
	if got["vague-instruction"] == 0 {
		t.Error("expected a vague-instruction finding for 'properly'")
	}
	if got["dead-import"] == 0 {
		t.Error("expected a dead-import finding")
	}
	if got["duplicate-line"] < 2 {
		t.Errorf("expected duplicate-line findings in both files, got %d", got["duplicate-line"])
	}
}

func TestLintFileTooLong(t *testing.T) {
	stack := &memory.ResolvedStack{
		Sources: []memory.Source{
			{Path: "/big/CLAUDE.md", Raw: strings.Repeat("a line\n", 250)},
		},
	}
	var found bool
	for _, f := range Lint(stack) {
		if f.Rule == "file-too-long" {
			found = true
		}
	}
	if !found {
		t.Error("expected a file-too-long finding for a 250-line file")
	}
}

func TestContainsWordBoundaries(t *testing.T) {
	if containsWord("scrub the area", "good") {
		t.Error("should not match 'good' inside other words")
	}
	if !containsWord("looks good to me", "good") {
		t.Error("should match standalone 'good'")
	}
}
