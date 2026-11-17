package verify

import (
	"strings"
	"testing"
	"unicode"

	"github.com/rick-learns/citebound/internal/corpus"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		line string
		want Claim
	}{
		{
			`Refresh tokens stay with the AS. [RFC6749 §1.5 ¶2: "are never sent to resource servers"]`,
			Claim{Text: "Refresh tokens stay with the AS.", Citation: "RFC6749 §1.5 ¶2", Ref: refreshRef, Quote: "are never sent to resource servers"},
		},
		{
			`- A bulleted claim. [RFC 6749 §1.5 ¶2 : "quote"]`,
			Claim{Text: "A bulleted claim.", Citation: "RFC 6749 §1.5 ¶2", Ref: refreshRef, Quote: "quote"},
		},
		{
			`Quotes inside quotes. [RFC6749 §1.5 ¶2: "the "code" value"]`,
			Claim{Text: "Quotes inside quotes.", Citation: "RFC6749 §1.5 ¶2", Ref: refreshRef, Quote: `the "code" value`},
		},
		{
			`Mentions [RFC6749] in passing. [RFC6749 §1.5 ¶2: "quote"]`,
			Claim{Text: "Mentions [RFC6749] in passing.", Citation: "RFC6749 §1.5 ¶2", Ref: refreshRef, Quote: "quote"},
		},
		{
			`Bad citation. [RFC6749 section 1.5: "quote"]`,
			Claim{Text: "Bad citation.", Citation: "RFC6749 section 1.5", Quote: "quote"},
		},
		{
			`No citation at all.`,
			Claim{Text: "No citation at all."},
		},
	}
	for _, tt := range tests {
		got, ok := ParseLine(tt.line)
		if !ok || got != tt.want {
			t.Errorf("ParseLine(%q) =\n %+v, %v\nwant\n %+v", tt.line, got, ok, tt.want)
		}
	}
	if _, ok := ParseLine("   "); ok {
		t.Error("blank line: ok = true, want false")
	}
}

func TestParseAnswer(t *testing.T) {
	claims, abstained := ParseAnswer("\nOne. [RFC6749 §1.5 ¶2: \"q\"]\n\nTwo.\n")
	if abstained || len(claims) != 2 {
		t.Fatalf("got %d claims, abstained=%v; want 2, false", len(claims), abstained)
	}
	claims, abstained = ParseAnswer("  " + AbstainMarker + "  \n")
	if !abstained || len(claims) != 0 {
		t.Fatalf("got %d claims, abstained=%v; want 0, true", len(claims), abstained)
	}
}

func FuzzFormatRoundTrip(f *testing.F) {
	f.Add("A claim.", "a quote with \"inner\" quotes", 6749, "10.4", 2)
	f.Fuzz(func(t *testing.T, text, quote string, rfc int, section string, para int) {
		c := Claim{Text: text, Quote: quote, Ref: corpus.Ref{RFC: rfc, Section: section, Paragraph: para}}
		got, _ := ParseLine(Format(c)) // must never panic
		if !canRoundTrip(c) {
			return
		}
		if got.Quote != c.Quote || got.Ref != c.Ref || got.Text != c.Text {
			t.Errorf("round trip of %+v gave %+v", c, got)
		}
	})
}

// canRoundTrip reports whether c is a claim a well-behaved answerer could
// produce: a valid ref, text that starts with a letter or digit, and no line
// breaks, brackets, or surrounding space in the text or quote.
func canRoundTrip(c Claim) bool {
	if _, err := corpus.ParseRef(c.Ref.String()); err != nil {
		return false
	}
	if c.Text == "" || !unicode.IsLetter(rune(c.Text[0])) && !unicode.IsDigit(rune(c.Text[0])) {
		return false
	}
	for _, s := range []string{c.Text, c.Quote} {
		if strings.ContainsAny(s, "\n\r[]") || s != strings.TrimSpace(s) {
			return false
		}
	}
	return true
}
