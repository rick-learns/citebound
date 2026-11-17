package answer

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/verify"
)

func TestSentences(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"One sentence here.", []string{"One sentence here."}},
		{"First one. Second one! Third one? fourth", []string{"First one.", "Second one!", "Third one? fourth"}},
		{"Use a nonce, e.g. a random value. Then stop.", []string{"Use a nonce, e.g. a random value.", "Then stop."}},
		{`It says "stop." Then it stops.`, []string{`It says "stop."`, "Then it stops."}},
		{"See Section 4.1. The client then retries.", []string{"See Section 4.1.", "The client then retries."}},
		{"Version 1.0 of the spec. 2 clients exist.", []string{"Version 1.0 of the spec.", "2 clients exist."}},
		{"Values such as client_id. lowercase continues.", []string{"Values such as client_id. lowercase continues."}},
		{"", nil},
	}
	for _, tt := range tests {
		if got := Sentences(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("Sentences(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func hit(ref string, text string, figure bool) index.Hit {
	r, err := corpus.ParseRef(ref)
	if err != nil {
		panic(err)
	}
	return index.Hit{Ref: r, Text: text, Figure: figure}
}

// mapLookup serves paragraph text for verification.
type mapLookup map[string]string

func (m mapLookup) Paragraph(ref corpus.Ref) (string, bool) {
	s, ok := m[ref.String()]
	return s, ok
}

func TestExtractive(t *testing.T) {
	passages := []index.Hit{
		hit("RFC1 §1 ¶1", "Clients are registered in advance. Refresh tokens rotate on every use.", false),
		hit("RFC1 §2 ¶1", "  +-------+\n  | refresh token rotation |\n  +-------+", true),
		hit("RFC1 §3 ¶1", "o Servers SHOULD rotate refresh tokens for public clients. Short ones.", false),
		hit("RFC1 §4 ¶1", "Nothing relevant is said in this paragraph at all.", false),
	}
	lookup := mapLookup{}
	for _, p := range passages {
		lookup[p.Ref.String()] = p.Text
	}

	got, err := Extractive{}.Answer(context.Background(), "How are refresh tokens rotated for public clients?", passages)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		`Servers SHOULD rotate refresh tokens for public clients. [RFC1 §3 ¶1: "Servers SHOULD rotate refresh tokens for public clients."]`,
		`Refresh tokens rotate on every use. [RFC1 §1 ¶1: "Refresh tokens rotate on every use."]`,
	}, "\n")
	if got != want {
		t.Fatalf("Answer =\n%s\nwant\n%s", got, want)
	}
	claims, abstained := verify.ParseAnswer(got)
	if abstained {
		t.Fatal("abstained")
	}
	for _, r := range verify.VerifyAll(lookup, claims) {
		if r.Status != verify.Verified {
			t.Errorf("claim %q: %s (%s)", r.Claim.Text, r.Status, r.Reason)
		}
	}

	one, _ := Extractive{MaxClaims: 1}.Answer(context.Background(), "How are refresh tokens rotated for public clients?", passages)
	if strings.Count(one, "\n") != 0 || !strings.Contains(one, "RFC1 §3 ¶1") {
		t.Errorf("MaxClaims 1: %q", one)
	}
	firstOnly, _ := Extractive{MaxPassages: 1}.Answer(context.Background(), "How are refresh tokens rotated for public clients?", passages)
	if strings.Contains(firstOnly, "RFC1 §3") {
		t.Errorf("MaxPassages 1 read the third passage: %q", firstOnly)
	}

	// Sentences the verifier or the answer format would reject are never
	// quoted: too short once the trailing period is ignored, and one with
	// text the format would read as a citation.
	for _, text := range []string{
		"Prevent token abuse.",
		`Prevent abuse of a token with [name: "value"] in it.`,
	} {
		tricky := []index.Hit{hit("RFC2 §1 ¶1", text, false)}
		if got, _ := (Extractive{}).Answer(context.Background(), "prevent token abuse", tricky); got != verify.AbstainMarker {
			t.Errorf("%q: answered %q, want abstain", text, got)
		}
	}

	// Figures do not use up the passage budget.
	figuresFirst := []index.Hit{
		hit("RFC3 §1 ¶1", "refresh tokens rotate", true),
		hit("RFC3 §1 ¶2", "refresh tokens rotate", true),
		passages[0],
	}
	if got, _ := (Extractive{MaxPassages: 1}).Answer(context.Background(), "refresh tokens", figuresFirst); !strings.Contains(got, "RFC1 §1 ¶1") {
		t.Errorf("figures first: %q, want a quote from RFC1 §1 ¶1", got)
	}

	for _, q := range []string{"What is the capital of France?", "what is the", ""} {
		if got, _ := (Extractive{}).Answer(context.Background(), q, passages); got != verify.AbstainMarker {
			t.Errorf("Answer(%q) = %q, want abstain", q, got)
		}
	}
	if got, _ := (Extractive{}).Answer(context.Background(), "refresh tokens", nil); got != verify.AbstainMarker {
		t.Errorf("no passages: %q, want abstain", got)
	}
}

var (
	pinnedOnce   sync.Once
	pinnedIndex  *index.Index
	pinnedLookup verify.CorpusLookup
	pinnedErr    error
)

func pinned(t *testing.T) (*index.Index, verify.CorpusLookup) {
	t.Helper()
	pinnedOnce.Do(func() {
		var rfcs []*corpus.RFC
		if rfcs, pinnedErr = corpus.Load("../../corpus"); pinnedErr != nil {
			return
		}
		pinnedLookup = verify.NewCorpusLookup(rfcs)
		pinnedIndex, pinnedErr = index.Build(context.Background(), rfcs)
	})
	if pinnedErr != nil {
		t.Fatal(pinnedErr)
	}
	return pinnedIndex, pinnedLookup
}

func TestExtractiveAgainstPinnedCorpus(t *testing.T) {
	ix, lookup := pinned(t)
	tests := []struct {
		question string
		wantRef  string // a section that should be cited; "" means abstain
	}{
		{"Can a refresh token be bound to a client?", "RFC8705 §7.1"},
		{"Should access tokens be sent in the URI query?", "RFC6750 §2.3"},
		{"How do I bake sourdough bread?", ""},
		{"What is the capital of France?", ""},
	}
	for _, tt := range tests {
		hits, err := ix.Search(context.Background(), tt.question, 8)
		if err != nil {
			t.Fatal(err)
		}
		answer, err := Extractive{}.Answer(context.Background(), tt.question, hits)
		if err != nil {
			t.Fatal(err)
		}
		claims, abstained := verify.ParseAnswer(answer)
		if tt.wantRef == "" {
			if !abstained || len(claims) != 0 {
				t.Errorf("%q: answered %q, want abstain", tt.question, answer)
			}
			continue
		}
		if !strings.Contains(answer, tt.wantRef+" ") {
			t.Errorf("%q: answer does not cite %s:\n%s", tt.question, tt.wantRef, answer)
		}
		for _, r := range verify.VerifyAll(lookup, claims) {
			if r.Status != verify.Verified {
				t.Errorf("%q: claim %q struck: %s", tt.question, r.Claim.Text, r.Reason)
			}
		}
	}
}
