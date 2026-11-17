// Package answer turns a question and retrieved passages into an answer in
// the format internal/verify checks.
package answer

import (
	"context"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/verify"
)

// An Answerer writes an answer to question from passages, best first. The
// answer is plain text in the answer format; it is parsed and verified
// afterwards, whoever wrote it.
type Answerer interface {
	Answer(ctx context.Context, question string, passages []index.Hit) (Reply, error)
}

// Reply is an answer and the model tokens it used, which are zero for an
// answerer without a model.
type Reply struct {
	Text                      string
	InputTokens, OutputTokens int
}

// Extractive answers by quoting whole sentences from the passages that cover
// the most words of the question. It needs no model and no network, so it
// powers the keyless demo and is the baseline every model is compared with.
//
// Each claim is a sentence quoted verbatim, and a sentence is only quoted if
// the claim passes verification. What it cannot do is decide whether a sentence answers the
// question; that is what the evals measure.
type Extractive struct {
	// MaxClaims caps the number of sentences quoted. Zero means 3.
	MaxClaims int
	// MaxPassages is how many of the best passages are read. Zero means 5.
	MaxPassages int
}

// candidate is a sentence that might be quoted.
type candidate struct {
	sentence string
	passage  int // rank of its passage, best first
	covered  int // distinct question terms it contains
}

// Answer implements Answerer.
func (e Extractive) Answer(_ context.Context, question string, passages []index.Hit) (Reply, error) {
	return Reply{Text: e.text(question, passages)}, nil
}

func (e Extractive) text(question string, passages []index.Hit) string {
	maxClaims := cmpOr(e.MaxClaims, 3)
	maxPassages := cmpOr(e.MaxPassages, 5)

	terms := stems(index.Terms(question))
	need := minCoverage(len(terms))
	if need == 0 {
		return verify.AbstainMarker
	}

	// The best sentence of each of the first maxPassages prose passages that
	// covers enough of the question.
	var picks []candidate
	read := 0
	for i, p := range passages {
		if read == maxPassages {
			break
		}
		if p.Figure {
			continue // figures are evidence, but not sentences to quote as claims
		}
		read++
		best := candidate{}
		for _, s := range Sentences(p.Text) {
			s = trimListMarker(s)
			if n := coverage(s, terms); n >= need && n > best.covered && survives(p, s) {
				best = candidate{sentence: s, passage: i, covered: n}
			}
		}
		if best.covered > 0 {
			picks = append(picks, best)
		}
	}
	if len(picks) == 0 {
		return verify.AbstainMarker
	}
	// Most of the question first; ties go to the better passage.
	slices.SortStableFunc(picks, func(a, b candidate) int {
		if a.covered != b.covered {
			return b.covered - a.covered
		}
		return a.passage - b.passage
	})
	picks = picks[:min(len(picks), maxClaims)]

	lines := make([]string, len(picks))
	for i, c := range picks {
		lines[i] = verify.Format(verify.Claim{Text: c.sentence, Ref: passages[c.passage].Ref, Quote: c.sentence})
	}
	return strings.Join(lines, "\n")
}

func cmpOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// minCoverage is how many distinct question terms a sentence must contain:
// all of a one- or two-term question, and at least half of a longer one.
func minCoverage(terms int) int {
	if terms <= 2 {
		return terms
	}
	return (terms + 1) / 2
}

// trimListMarker drops a leading bullet such as "o " or "* ", which RFCs use
// for list items. The rest is still a verbatim substring of the paragraph.
func trimListMarker(s string) string {
	for _, m := range []string{"o ", "* ", "- "} {
		if rest, ok := strings.CutPrefix(s, m); ok {
			return rest
		}
	}
	return s
}

// survives reports whether quoting sentence s from passage p gives a claim
// that parses back unchanged and verifies. Checking with the verifier itself,
// rather than re-deriving its rules, keeps the promise that every extractive
// claim verifies: a sentence that is too short after normalization, or that
// contains text like [name: "value"] that the answer format would misread,
// is never quoted.
func survives(p index.Hit, s string) bool {
	c := verify.Claim{Text: s, Ref: p.Ref, Quote: s}
	parsed, ok := verify.ParseLine(verify.Format(c))
	if !ok || parsed.Text != s || parsed.Quote != s || parsed.Ref != p.Ref {
		return false
	}
	return verify.Verify(passageLookup{p}, parsed).Status == verify.Verified
}

// passageLookup serves the text of one passage to the verifier.
type passageLookup struct{ p index.Hit }

func (l passageLookup) Paragraph(ref corpus.Ref) (string, bool) {
	return l.p.Text, ref == l.p.Ref
}

func coverage(sentence string, terms []string) int {
	words := map[string]bool{}
	for _, w := range stems(index.Terms(sentence)) {
		words[w] = true
	}
	n := 0
	for _, t := range terms {
		if words[t] {
			n++
		}
	}
	return n
}

// stems reduces words to a crude stem so "tokens" matches "token" and
// "rotating" matches "rotate". It only needs to agree with itself.
func stems(words []string) []string {
	seen := map[string]bool{}
	out := words[:0:0]
	for _, w := range words {
		w = stem(w)
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func stem(w string) string {
	for _, suffix := range []string{"ing", "ed", "es", "s", "e"} {
		if len(w)-len(suffix) >= 4 && strings.HasSuffix(w, suffix) {
			return strings.TrimSuffix(w, suffix)
		}
	}
	return w
}

// abbreviations end in a period without ending a sentence.
var abbreviations = map[string]bool{"e.g.": true, "i.e.": true, "etc.": true, "cf.": true, "vs.": true, "al.": true, "Sec.": true}

// Sentences splits prose into sentences. A sentence ends at ".", "!", or "?"
// (optionally followed by a closing quote or parenthesis) when the next word
// starts with an upper-case letter, a digit, or an opening quote, and the
// word before is not an abbreviation such as "e.g.".
func Sentences(text string) []string {
	words := strings.Fields(text)
	var out []string
	start := 0
	for i, w := range words {
		if i+1 < len(words) && !endsSentence(w, words[i+1]) {
			continue
		}
		out = append(out, strings.Join(words[start:i+1], " "))
		start = i + 1
	}
	return out
}

func endsSentence(word, next string) bool {
	if abbreviations[word] {
		return false
	}
	trimmed := strings.TrimRight(word, `")'`)
	if trimmed == "" || !strings.ContainsAny(trimmed[len(trimmed)-1:], ".!?") {
		return false
	}
	r, _ := utf8.DecodeRuneInString(next)
	return unicode.IsUpper(r) || unicode.IsDigit(r) || r == '"' || r == '('
}
