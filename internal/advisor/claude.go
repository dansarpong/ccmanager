package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrClaudeUnavailable means the local claude CLI could not be used (not
// installed, errored, or not logged in). Callers should fall back to the
// always-available static lint and show a clear banner. The static lint never
// depends on claude.
var ErrClaudeUnavailable = errors.New("claude CLI unavailable")

// Analysis kinds the advisor asks claude for.
const (
	KindRewrite       = "rewrite"
	KindContradiction = "contradiction"
)

// resultEnvelope is the JSON shape `claude -p --output-format json` prints.
// Verified against v2.1.186: on success .result holds the assistant text; on
// failure .is_error is true and .result holds the error message (e.g.
// "Not logged in · Please run /login").
type resultEnvelope struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
}

// claudeRunner runs the CLI; swapped out in tests.
type claudeRunner interface {
	run(ctx context.Context, stdin string) (resultEnvelope, error)
}

// cliRunner shells out to the real claude binary with no tools granted, so it
// can only analyze the text passed on stdin and cannot touch the filesystem.
type cliRunner struct {
	bin          string
	systemPrompt string
}

func (c cliRunner) run(ctx context.Context, stdin string) (resultEnvelope, error) {
	args := []string{
		"-p",
		"--output-format", "json",
		"--bare",
		"--tools", "",
		"--allowed-tools", "",
	}
	if c.systemPrompt != "" {
		args = append(args, "--append-system-prompt", c.systemPrompt)
	}
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		// A non-zero exit with parseable JSON still tells us is_error/result.
		if env, perr := parseEnvelope(out.Bytes()); perr == nil {
			return env, nil
		}
		return resultEnvelope{}, fmt.Errorf("%w: %v: %s", ErrClaudeUnavailable, err, strings.TrimSpace(errOut.String()))
	}
	env, err := parseEnvelope(out.Bytes())
	if err != nil {
		return resultEnvelope{}, fmt.Errorf("%w: %v", ErrClaudeUnavailable, err)
	}
	return env, nil
}

func parseEnvelope(b []byte) (resultEnvelope, error) {
	var env resultEnvelope
	if err := json.Unmarshal(bytes.TrimSpace(b), &env); err != nil {
		return resultEnvelope{}, fmt.Errorf("parsing claude output: %w", err)
	}
	return env, nil
}

// callClaude runs the given runner and returns the assistant text, mapping an
// errored envelope (e.g. not logged in) to ErrClaudeUnavailable.
func callClaude(ctx context.Context, r claudeRunner, stdin string) (string, error) {
	env, err := r.run(ctx, stdin)
	if err != nil {
		return "", err
	}
	if env.IsError {
		return "", fmt.Errorf("%w: %s", ErrClaudeUnavailable, strings.TrimSpace(env.Result))
	}
	return env.Result, nil
}
