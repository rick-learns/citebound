// Package verify checks that every quote in an answer appears, word for
// word, in the paragraph it cites.
//
// Verification is deterministic: no model calls, no randomness, and no
// network access. The same answer against the same corpus always produces
// the same result.
package verify

import (
	"strings"
	"unicode/utf8"

	"github.com/rick-learns/citebound/internal/corpus"
)

// Minimum quote size. Shorter quotes ("MUST", "the client") match almost
// anywhere and prove nothing.
const (
	MinQuoteRunes = 20
	MinQuoteWords = 3
)

// Claim is one sentence of an answer with the citation that supports it.
type Claim struct {
	Text  string
	Ref   corpus.Ref
	Quote string
	// Citation is the citation as written, kept for reporting when it does
	// not parse.
	Citation string
}

// Status is the outcome of verifying one claim.
type Status string

// Verification outcomes.
const (
	Verified Status = "verified"
	Struck   Status = "struck"
)

// Reason explains why a claim was struck.
type Reason string

// Reasons a claim is struck.
const (
	ReasonNone          Reason = ""
	ReasonUncited       Reason = "uncited"         // no citation at all
	ReasonBadCitation   Reason = "bad-citation"    // citation is not in the RFC6749 §10.4 ¶2 form
	ReasonUnknownRef    Reason = "unknown-ref"     // the cited paragraph does not exist
	ReasonQuoteTooShort Reason = "quote-too-short" // quote is too short to be evidence
	ReasonQuoteNotFound Reason = "quote-not-found" // quote is not in the cited paragraph
)

// Result is the verdict for one claim.
type Result struct {
	Claim  Claim
	Status Status
	Reason Reason
}

// Lookup returns the text of a paragraph, or false if it does not exist.
type Lookup interface {
	Paragraph(corpus.Ref) (string, bool)
}

// Verify checks one claim against the corpus.
func Verify(l Lookup, c Claim) Result {
	struck := func(r Reason) Result { return Result{Claim: c, Status: Struck, Reason: r} }
	if c.Citation == "" && c.Ref == (corpus.Ref{}) {
		return struck(ReasonUncited)
	}
	if c.Ref == (corpus.Ref{}) {
		return struck(ReasonBadCitation)
	}
	text, ok := l.Paragraph(c.Ref)
	if !ok {
		return struck(ReasonUnknownRef)
	}
	quote := Normalize(c.Quote)
	if !bigEnough(quote) {
		return struck(ReasonQuoteTooShort)
	}
	text = Normalize(text)
	if !strings.Contains(text, quote) {
		// The size rule applies to the form that matched: backslashes are
		// not evidence.
		unescaped := unescapeQuotes(quote)
		if unescaped == quote || !strings.Contains(text, unescaped) {
			return struck(ReasonQuoteNotFound)
		}
		if !bigEnough(unescaped) {
			return struck(ReasonQuoteTooShort)
		}
	}
	return Result{Claim: c, Status: Verified}
}

func bigEnough(quote string) bool {
	return utf8.RuneCountInString(quote) >= MinQuoteRunes && len(strings.Fields(quote)) >= MinQuoteWords
}

// unescapeQuotes reads \" as ". Models escape double quotes inside a quote,
// as in "the \"aud\" claim", even when told not to. Verify tries the quote as
// written first, because a few RFC examples contain a literal \"; either way
// the quote must be in the cited paragraph. See decision 0008.
func unescapeQuotes(s string) string {
	return strings.ReplaceAll(s, `\"`, `"`)
}

// VerifyAll checks every claim, in order.
func VerifyAll(l Lookup, claims []Claim) []Result {
	out := make([]Result, len(claims))
	for i, c := range claims {
		out[i] = Verify(l, c)
	}
	return out
}

var typographic = strings.NewReplacer(
	"\u2018", "'", "\u2019", "'", // single quotes
	"\u201c", `"`, "\u201d", `"`, // double quotes
	"\u2013", "-", "\u2014", "-", "\u2011", "-", // dashes and non-breaking hyphen
	"\u00a0", " ", // non-breaking space
)

// Normalize prepares text for comparison. It maps typographic quotes and
// dashes to ASCII, collapses whitespace, and trims surrounding whitespace and
// trailing sentence punctuation. It never changes letters or their case: RFC
// keywords such as MUST and must mean different things.
func Normalize(s string) string {
	s = typographic.Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimRight(s, ".,;:")
}

// CorpusLookup serves paragraphs from parsed RFCs.
type CorpusLookup map[int]*corpus.RFC

// NewCorpusLookup indexes parsed RFCs by number.
func NewCorpusLookup(rfcs []*corpus.RFC) CorpusLookup {
	l := make(CorpusLookup, len(rfcs))
	for _, r := range rfcs {
		l[r.Number] = r
	}
	return l
}

// Paragraph implements Lookup.
func (l CorpusLookup) Paragraph(ref corpus.Ref) (string, bool) {
	r, ok := l[ref.RFC]
	if !ok {
		return "", false
	}
	p := r.Paragraph(ref)
	if p == nil {
		return "", false
	}
	return p.Text, true
}
