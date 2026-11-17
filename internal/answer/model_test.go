package answer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/llm"
	"github.com/rick-learns/citebound/internal/verify"
)

// scripted is a Provider that returns a fixed reply and keeps the request.
type scripted struct {
	reply string
	err   error
	got   *llm.Request
}

func (s *scripted) Name() string { return "test/scripted" }

func (s *scripted) Complete(_ context.Context, req llm.Request) (*llm.Response, error) {
	s.got = &req
	if s.err != nil {
		return nil, s.err
	}
	return &llm.Response{Text: s.reply}, nil
}

func TestModelRequest(t *testing.T) {
	passages := []index.Hit{
		{Ref: corpus.Ref{RFC: 6749, Section: "6", Paragraph: 5}, SectionTitle: "Refreshing an Access Token", Text: "the refresh token is bound to the client"},
		{Ref: corpus.Ref{RFC: 7636, Section: "4.1", Paragraph: 2}, SectionTitle: "Client Creates a Code Verifier", Text: "code-verifier = 43*128unreserved", Figure: true},
	}
	req := Model{}.Request("  Is it bound?\n", passages)
	want := "Question: Is it bound?\n\nPassages:\n" +
		"\n[RFC6749 §6 ¶5] (RFC 6749, Refreshing an Access Token)\nthe refresh token is bound to the client\n" +
		"\n[RFC7636 §4.1 ¶2] (RFC 7636, Client Creates a Code Verifier, figure)\ncode-verifier = 43*128unreserved\n"
	if req.User != want {
		t.Errorf("User =\n%s\nwant\n%s", req.User, want)
	}
	if req.System != SystemPrompt || req.MaxTokens != 4096 {
		t.Errorf("System or MaxTokens wrong: %d", req.MaxTokens)
	}
	if got := (Model{MaxTokens: 50}).Request("q", passages).MaxTokens; got != 50 {
		t.Errorf("MaxTokens = %d, want 50", got)
	}
}

// The example in the prompt must itself pass verification, or the model is
// taught a format the verifier rejects.
func TestSystemPromptExampleVerifies(t *testing.T) {
	_, lookup := pinned(t)
	var example string
	for _, line := range strings.Split(SystemPrompt, "\n") {
		if strings.Contains(line, `[RFC6749 §1.5 ¶2: "`) {
			example = line
		}
	}
	claims, _ := verify.ParseAnswer(example)
	if len(claims) != 1 {
		t.Fatalf("example %q parsed as %d claims", example, len(claims))
	}
	if r := verify.Verify(lookup, claims[0]); r.Status != verify.Verified {
		t.Errorf("example is struck: %s", r.Reason)
	}
}

func TestModelAnswer(t *testing.T) {
	hit := []index.Hit{{Ref: corpus.Ref{RFC: 6749, Section: "1", Paragraph: 1}, Text: "text"}}
	tests := []struct {
		name, reply, want string
	}{
		{"plain", "A. [RFC6749 §1 ¶1: \"q\"]\n", "A. [RFC6749 §1 ¶1: \"q\"]\n"},
		{"fenced", "```text\nA. [RFC6749 §1 ¶1: \"q\"]\nB.\n```\n", "A. [RFC6749 §1 ¶1: \"q\"]\nB."},
		{"bare fence", "```\nINSUFFICIENT_EVIDENCE\n```", "INSUFFICIENT_EVIDENCE"},
		{"fence only at start", "```\nA.", "```\nA."},
		{"backticks only", "``````", "``````"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Model{Provider: &scripted{reply: tt.reply}}.Answer(context.Background(), "q", hit)
			if err != nil || got != tt.want {
				t.Errorf("Answer = %q, %v; want %q", got, err, tt.want)
			}
		})
	}

	p := &scripted{reply: "unused"}
	if got, err := (Model{Provider: p}).Answer(context.Background(), "q", nil); got != verify.AbstainMarker || err != nil || p.got != nil {
		t.Errorf("no passages: %q, %v, called=%v; want the abstain marker without a model call", got, err, p.got != nil)
	}

	boom := errors.New("boom")
	if _, err := (Model{Provider: &scripted{err: boom}}).Answer(context.Background(), "q", hit); !errors.Is(err, boom) {
		t.Errorf("provider error: %v", err)
	}
}

// A model reply goes through the same verifier as any other answer: a real
// quote verifies, and an altered one is struck.
func TestModelInPipeline(t *testing.T) {
	ix, lookup := pinned(t)
	reply := strings.Join([]string{
		`A refresh token is bound to the client it was issued to. [RFC6749 §6 ¶5: "the refresh token is bound to the client to which it was issued"]`,
		`Refresh tokens are never bound to a client. [RFC6749 §6 ¶5: "the refresh token is never bound to the client to which it was issued"]`,
	}, "\n")
	p := &Pipeline{Index: ix, Lookup: lookup, Answerer: Model{Provider: &scripted{reply: reply}}, Policy: DefaultPolicy}
	res, err := p.Ask(context.Background(), "Can a refresh token be bound to a client?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Claims) != 2 || res.Claims[0].Status != verify.Verified || res.Claims[1].Reason != verify.ReasonQuoteNotFound {
		t.Fatalf("claims = %+v", res.Claims)
	}
	if res.Abstained {
		t.Errorf("abstained (%s) with one of two claims verified", res.Reason)
	}
}
