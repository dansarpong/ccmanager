package advisor

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubRunner returns a canned envelope, recording how many times it ran (to
// prove caching avoids a second call).
type stubRunner struct {
	env   resultEnvelope
	err   error
	calls int
}

func (s *stubRunner) run(_ context.Context, _ string) (resultEnvelope, error) {
	s.calls++
	return s.env, s.err
}

// memCache is an in-memory Cache for tests.
type memCache struct {
	data map[string]string
}

func newMemCache() *memCache { return &memCache{data: map[string]string{}} }

func (m *memCache) GetAdvisorCache(hash, kind string) (string, bool, error) {
	v, ok := m.data[kind+":"+hash]
	return v, ok, nil
}
func (m *memCache) PutAdvisorCache(_, kind, hash, result, _ string) error {
	m.data[kind+":"+hash] = result
	return nil
}

func newTestAdvisor(r claudeRunner, c Cache) *Advisor {
	return &Advisor{runner: r, cache: c, clock: func() time.Time { return time.Unix(0, 0) }}
}

func TestRewriteParsesSuggestions(t *testing.T) {
	runner := &stubRunner{env: resultEnvelope{
		Result: `[{"original":"format properly","suggestion":"use 2-space indent","reason":"concrete"}]`,
	}}
	a := newTestAdvisor(runner, newMemCache())
	sugs, err := a.RewriteFile(context.Background(), "/p/CLAUDE.md", "format properly")
	if err != nil {
		t.Fatalf("RewriteFile: %v", err)
	}
	if len(sugs) != 1 || sugs[0].Suggestion != "use 2-space indent" || sugs[0].File != "/p/CLAUDE.md" {
		t.Errorf("unexpected suggestions: %+v", sugs)
	}
}

func TestRewriteToleratesProseAroundJSON(t *testing.T) {
	runner := &stubRunner{env: resultEnvelope{
		Result: "Here are my suggestions:\n[{\"original\":\"x\",\"suggestion\":\"y\",\"reason\":\"z\"}]\nHope that helps!",
	}}
	a := newTestAdvisor(runner, newMemCache())
	sugs, err := a.RewriteFile(context.Background(), "/p/CLAUDE.md", "x")
	if err != nil || len(sugs) != 1 {
		t.Fatalf("expected 1 suggestion, got %d err=%v", len(sugs), err)
	}
}

func TestErrorEnvelopeBecomesUnavailable(t *testing.T) {
	runner := &stubRunner{env: resultEnvelope{IsError: true, Result: "Not logged in · Please run /login"}}
	a := newTestAdvisor(runner, newMemCache())
	_, err := a.RewriteFile(context.Background(), "/p/CLAUDE.md", "x")
	if !errors.Is(err, ErrClaudeUnavailable) {
		t.Errorf("expected ErrClaudeUnavailable, got %v", err)
	}
}

func TestCacheAvoidsSecondCall(t *testing.T) {
	runner := &stubRunner{env: resultEnvelope{Result: `[]`}}
	cache := newMemCache()
	a := newTestAdvisor(runner, cache)

	if _, err := a.RewriteFile(context.Background(), "/p/CLAUDE.md", "same content"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RewriteFile(context.Background(), "/p/CLAUDE.md", "same content"); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Errorf("expected 1 claude call thanks to caching, got %d", runner.calls)
	}
}

func TestContradictionsParse(t *testing.T) {
	runner := &stubRunner{env: resultEnvelope{
		Result: `[{"fileA":"/a","fileB":"/b","claimA":"always tabs","claimB":"always spaces","explanation":"conflict"}]`,
	}}
	a := newTestAdvisor(runner, newMemCache())
	// flattenStack needs a stack; build a tiny one indirectly via Contradictions
	// is awkward, so call the prompt path through a minimal stack.
	got, err := a.contradictionsFromDoc(context.Background(), "/work", "=== FILE: /a ===\n...")
	if err != nil || len(got) != 1 || got[0].Explanation != "conflict" {
		t.Errorf("contradictions = %+v err=%v", got, err)
	}
}
