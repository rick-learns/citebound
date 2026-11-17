package verify

import (
	"testing"

	"github.com/rick-learns/citebound/internal/corpus"
)

type fakeLookup map[corpus.Ref]string

func (f fakeLookup) Paragraph(r corpus.Ref) (string, bool) {
	t, ok := f[r]
	return t, ok
}

var refreshRef = corpus.Ref{RFC: 6749, Section: "1.5", Paragraph: 2}

var lookup = fakeLookup{
	refreshRef: "authorization information. Unlike access tokens, refresh tokens are intended for use only with authorization servers and are never sent to resource servers.",
	{RFC: 6749, Section: "4.1", Paragraph: 1}: "+----------+\n| Resource |\n|  Owner   |\n+----------+",
	audRef:     `The resource server MUST validate that the "aud" claim contains a resource indicator value`,
	literalRef: `WWW-Authenticate: Basic realm="simple", title="Login to \"apps\""`,
}

var (
	audRef     = corpus.Ref{RFC: 9068, Section: "4", Paragraph: 8}
	literalRef = corpus.Ref{RFC: 9110, Section: "11.6.1", Paragraph: 6}
)

func TestVerify(t *testing.T) {
	tests := []struct {
		name  string
		claim Claim
		want  Reason
	}{
		{"exact quote", Claim{Ref: refreshRef, Citation: "x", Quote: "refresh tokens are intended for use only with authorization servers"}, ReasonNone},
		{"typographic quotes and extra spaces", Claim{Ref: refreshRef, Citation: "x", Quote: "Unlike access tokens,  refresh tokens are intended for use only"}, ReasonNone},
		{"trailing period", Claim{Ref: refreshRef, Citation: "x", Quote: "are never sent to resource servers."}, ReasonNone},
		{"case matters", Claim{Ref: refreshRef, Citation: "x", Quote: "Refresh Tokens are intended for use only with authorization servers"}, ReasonQuoteNotFound},
		{"paraphrase", Claim{Ref: refreshRef, Citation: "x", Quote: "refresh tokens are only used with authorization servers"}, ReasonQuoteNotFound},
		{"ellipsis is not allowed", Claim{Ref: refreshRef, Citation: "x", Quote: "refresh tokens are intended ... never sent to resource servers"}, ReasonQuoteNotFound},
		{"right quote, wrong paragraph", Claim{Ref: corpus.Ref{RFC: 6749, Section: "4.1", Paragraph: 1}, Citation: "x", Quote: "refresh tokens are intended for use only"}, ReasonQuoteNotFound},
		{"too few characters", Claim{Ref: refreshRef, Citation: "x", Quote: "are never sent"}, ReasonQuoteTooShort},
		{"too few words", Claim{Ref: refreshRef, Citation: "x", Quote: "authorization servers"}, ReasonQuoteTooShort},
		{"unknown paragraph", Claim{Ref: corpus.Ref{RFC: 6749, Section: "1.5", Paragraph: 99}, Citation: "x", Quote: "refresh tokens are intended for use only"}, ReasonUnknownRef},
		{"unparseable citation", Claim{Citation: "RFC 6749 section 1.5", Quote: "refresh tokens are intended for use only"}, ReasonBadCitation},
		{"no citation", Claim{Text: "Refresh tokens are great."}, ReasonUncited},
		{"double quotes as written", Claim{Ref: audRef, Citation: "x", Quote: `MUST validate that the "aud" claim contains`}, ReasonNone},
		{"escaped double quotes", Claim{Ref: audRef, Citation: "x", Quote: `MUST validate that the \"aud\" claim contains`}, ReasonNone},
		{"escaped quotes do not excuse other changes", Claim{Ref: audRef, Citation: "x", Quote: `MUST validate that the \"iss\" claim contains`}, ReasonQuoteNotFound},
		{"a literal backslash in the RFC", Claim{Ref: literalRef, Citation: "x", Quote: `title="Login to \"apps\""`}, ReasonNone},
		{"backslashes do not count toward the size", Claim{Ref: audRef, Citation: "x", Quote: `at the \"aud\" claim`}, ReasonQuoteTooShort}, // 20 runes as written, 18 unescaped
		{"just long enough once unescaped", Claim{Ref: audRef, Citation: "x", Quote: `that the \"aud\" claim`}, ReasonNone},                 // 20 runes unescaped
		{"other escapes are not read", Claim{Ref: audRef, Citation: "x", Quote: `MUST validate that the \'aud\' claim`}, ReasonQuoteNotFound},
		{"figure quote across lines", Claim{Ref: corpus.Ref{RFC: 6749, Section: "4.1", Paragraph: 1}, Citation: "x", Quote: "+----------+ | Resource | | Owner |"}, ReasonNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Verify(lookup, tt.claim)
			wantStatus := Verified
			if tt.want != ReasonNone {
				wantStatus = Struck
			}
			if got.Status != wantStatus || got.Reason != tt.want {
				t.Errorf("Verify() = %s/%q, want %s/%q", got.Status, got.Reason, wantStatus, tt.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		"  a\n  b\tc  ":          "a b c",
		"“quoted” ‘single’":      `"quoted" 'single'`,
		"long—dash – en‑hyph":    "long-dash - en-hyph",
		"ends with punctuation.": "ends with punctuation",
		"MUST not change":        "MUST not change",
	}
	for in, want := range tests {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVerifyAgainstPinnedCorpus(t *testing.T) {
	rfcs, err := corpus.Load("../../corpus")
	if err != nil {
		t.Fatal(err)
	}
	l := NewCorpusLookup(rfcs)
	answer := `PKCE derives the challenge from the verifier with SHA-256. [RFC7636 §4.2 ¶3: "code_challenge = BASE64URL-ENCODE(SHA256(ASCII(code_verifier)))"]
Refresh tokens never go to resource servers. [RFC6749 §1.5 ¶2: "refresh tokens are intended for use only with authorization servers and are never sent to resource servers"]
Refresh tokens must always be rotated. [RFC6749 §10.4 ¶1: "refresh tokens MUST always be rotated on every use"]
The token must contain an "exp" claim. [RFC9068 §4 ¶10: "The current time MUST be before the time represented by the \"exp\" claim."]`
	claims, _ := ParseAnswer(answer)
	got := VerifyAll(l, claims)
	want := []Status{Verified, Verified, Struck, Verified}
	if len(got) != len(want) {
		t.Fatalf("got %d results, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Status != want[i] {
			t.Errorf("claim %d (%s) = %s/%s, want %s", i+1, got[i].Claim.Ref, got[i].Status, got[i].Reason, want[i])
		}
	}
}
