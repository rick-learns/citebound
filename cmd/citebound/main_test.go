package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "") // the no-key case must not depend on the environment
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no args", args: nil, wantCode: 2, wantStderr: "Usage:"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "dev"},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "Commands:"},
		{name: "unknown", args: []string{"nope"}, wantCode: 2, wantStderr: `unknown command "nope"`},
		{name: "corpus verify", args: []string{"corpus", "verify", "-dir", "../../corpus"}, wantCode: 0, wantStdout: "21 RFCs verified"},
		{name: "corpus verify missing dir", args: []string{"corpus", "verify", "-dir", "does-not-exist"}, wantCode: 1, wantStderr: "read manifest"},
		{name: "corpus stats", args: []string{"corpus", "stats", "-dir", "../../corpus"}, wantCode: 0, wantStdout: "Total"},
		{name: "search", args: []string{"search", "-dir", "../../corpus", "-k", "1", "refresh", "token", "rotation"}, wantCode: 0, wantStdout: "1. RFC"},
		{name: "search without query", args: []string{"search"}, wantCode: 2, wantStderr: "Usage: citebound search"},
		{name: "search with only stopwords", args: []string{"search", "-dir", "../../corpus", "what is the"}, wantCode: 1, wantStderr: "no searchable words"},
		{name: "ask", args: []string{"ask", "-dir", "../../corpus", "Can a refresh token be bound to a client?"}, wantCode: 0, wantStdout: "✓ "},
		{name: "ask off topic", args: []string{"ask", "-dir", "../../corpus", "How do I bake sourdough bread?"}, wantCode: 0, wantStdout: "does not support an answer"},
		{name: "ask replayed model", args: []string{"ask", "-dir", "../../corpus", "-model", "openrouter/nvidia/nemotron-3-ultra-550b-a55b:free", "-replay", "../../evals/recordings/nemotron-3-ultra.jsonl", "Can a refresh token be bound to a client?"}, wantCode: 0, wantStdout: "✓ "},
		{name: "ask replayed model, escaped quotes", args: []string{"ask", "-dir", "../../corpus", "-model", "openrouter/nvidia/nemotron-3-ultra-550b-a55b:free", "-replay", "../../evals/recordings/nemotron-3-ultra.jsonl", "What claims must a JWT access token contain?"}, wantCode: 0, wantStdout: "✓ A JWT access token must include an \"aud\" claim"},
		{name: "ask replay miss", args: []string{"ask", "-dir", "../../corpus", "-model", "openrouter/nvidia/nemotron-3-ultra-550b-a55b:free", "-replay", "../../evals/recordings/nemotron-3-ultra.jsonl", "A question nobody recorded about tokens"}, wantCode: 1, wantStderr: "no recording for this request"},
		{name: "ask model without key", args: []string{"ask", "-model", "anthropic/claude-sonnet-5-5", "q"}, wantCode: 1, wantStderr: "needs ANTHROPIC_API_KEY"},
		{name: "ask unknown provider", args: []string{"ask", "-model", "acme/m", "q"}, wantCode: 1, wantStderr: "want extractive"},
		{name: "ask record and replay", args: []string{"ask", "-model", "openai/m", "-record", "a", "-replay", "b", "q"}, wantCode: 1, wantStderr: "not both"},
		{name: "ask replay without model", args: []string{"ask", "-replay", "b", "q"}, wantCode: 1, wantStderr: "need -model"},
		{name: "ask without question", args: []string{"ask"}, wantCode: 2, wantStderr: "Usage: citebound ask"},
		{name: "eval retrieval", args: []string{"eval", "retrieval", "-dir", "../../corpus", "-set", "../../evals/retrieval.jsonl", "-thresholds", "../../evals/thresholds.json"}, wantCode: 0, wantStdout: "Above the thresholds"},
		{name: "eval retrieval below thresholds", args: []string{"eval", "retrieval", "-dir", "../../corpus", "-set", "../../evals/retrieval.jsonl", "-thresholds", "testdata/high-thresholds.json"}, wantCode: 1, wantStderr: "below"},
		{name: "check mixed answer", args: []string{"check", "-dir", "../../corpus", "testdata/answer-mixed.txt"}, wantCode: 1, wantStdout: "struck: quote-not-found\n\n✗ Refresh tokens are defined in section 99.\n  struck: unknown-ref\n\n1 of 3 claims verified."},
		{name: "check verified answer", args: []string{"check", "-dir", "../../corpus", "testdata/answer-verified.txt"}, wantCode: 0, wantStdout: "1 of 1 claims verified."},
		{name: "check empty answer", args: []string{"check", "-dir", "../../corpus", "testdata/answer-blank.txt"}, wantCode: 1, wantStderr: "no claims in testdata/answer-blank.txt"},
		{name: "check missing file", args: []string{"check", "nope.txt"}, wantCode: 1, wantStderr: "nope.txt"},
		{name: "check without file", args: []string{"check"}, wantCode: 2, wantStderr: "Usage: citebound check"},
		{name: "eval without subcommand", args: []string{"eval"}, wantCode: 2, wantStderr: "Usage: citebound eval retrieval"},
		{name: "corpus without subcommand", args: []string{"corpus"}, wantCode: 2, wantStderr: "corpus verify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Errorf("exit code = %d, want %d", got, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestCheckStdin(t *testing.T) {
	for in, want := range map[string]string{
		"INSUFFICIENT_EVIDENCE\n": "The answer abstains.",
		`Refresh tokens only go to the authorization server. [RFC6749 §1.5 ¶2: "refresh tokens are intended for use only with authorization servers"]`: "1 of 1 claims verified.",
	} {
		stdin = strings.NewReader(in)
		var out, errOut bytes.Buffer
		if code := run([]string{"check", "-dir", "../../corpus", "-"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), want) {
			t.Errorf("check - <%q: code %d, stdout %q, stderr %q", in, code, out.String(), errOut.String())
		}
	}
	stdin = os.Stdin
}

func TestCheckSizeLimit(t *testing.T) {
	defer func() { stdin = os.Stdin }()
	for size, tooLarge := range map[int]bool{maxAnswerBytes: false, maxAnswerBytes + 1: true} {
		stdin = strings.NewReader(strings.Repeat("\n", size))
		var out, errOut bytes.Buffer
		code := run([]string{"check", "-dir", "../../corpus", "-"}, &out, &errOut)
		if code != 1 || strings.Contains(errOut.String(), "larger than") != tooLarge {
			t.Errorf("%d bytes: code %d, stderr %q", size, code, errOut.String())
		}
	}
}
