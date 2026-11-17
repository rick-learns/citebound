package answer

import (
	"context"
	"errors"
	"fmt"

	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/verify"
)

// Searcher is the part of the index the pipeline needs.
type Searcher interface {
	Search(ctx context.Context, query string, k int) ([]index.Hit, error)
}

// AbstainReason says why an answer was withheld.
type AbstainReason string

// Reasons to abstain, in the order the pipeline checks them. See decision
// 0006.
const (
	NoPassages       AbstainReason = "no-passages"        // retrieval found nothing to read
	AnswererAbstains AbstainReason = "answerer-abstained" // the answerer said the passages do not answer it
	NoVerifiedClaims AbstainReason = "no-verified-claims" // every claim was struck
	TooManyStruck    AbstainReason = "too-many-struck"    // fewer than MinVerifiedFraction of claims verified
)

// Policy decides when to withhold an answer.
type Policy struct {
	// MinVerifiedFraction is the share of claims that must verify. An answer
	// that is mostly fabricated is not trusted, even for the claims that
	// happen to check out.
	MinVerifiedFraction float64
}

// DefaultPolicy is the policy described in decision 0006.
var DefaultPolicy = Policy{MinVerifiedFraction: 0.5}

// Pipeline answers questions: retrieve, answer, verify, then decide.
type Pipeline struct {
	Index    Searcher
	Lookup   verify.Lookup
	Answerer Answerer
	Policy   Policy
	// Passages is how many search results the answerer reads. Zero means 8.
	Passages int
}

// Result is a finished answer. When Abstained is set, Claims still holds
// every claim and its verdict, for transparency, but none of them is the
// answer.
type Result struct {
	Passages  []index.Hit
	Claims    []verify.Result
	Verified  int
	Abstained bool
	Reason    AbstainReason
}

// Event reports progress while the pipeline runs, so callers can stream it.
// Exactly one field is set.
type Event struct {
	Passages []index.Hit    // retrieval finished
	Claim    *verify.Claim  // a claim was parsed; its verdict follows
	Verdict  *verify.Result // a claim was checked
}

// Ask answers question. emit, if not nil, receives events in order:
// passages, then a claim and its verdict for each claim.
func (p *Pipeline) Ask(ctx context.Context, question string, emit func(Event)) (*Result, error) {
	if emit == nil {
		emit = func(Event) {}
	}
	hits, err := p.Index.Search(ctx, question, cmpOr(p.Passages, 8))
	if err != nil && !errors.Is(err, index.ErrEmptyQuery) {
		return nil, fmt.Errorf("search: %w", err)
	}
	res := &Result{Passages: hits}
	emit(Event{Passages: hits})
	if len(hits) == 0 {
		return res.abstain(NoPassages), nil
	}

	text, err := p.Answerer.Answer(ctx, question, hits)
	if err != nil {
		return nil, fmt.Errorf("answer: %w", err)
	}
	claims, abstained := verify.ParseAnswer(text)
	for _, c := range claims {
		emit(Event{Claim: &c})
		v := verify.Verify(p.Lookup, c)
		emit(Event{Verdict: &v})
		res.Claims = append(res.Claims, v)
		if v.Status == verify.Verified {
			res.Verified++
		}
	}
	return p.Policy.decide(res, abstained), nil
}

func (pol Policy) decide(res *Result, answererAbstained bool) *Result {
	switch {
	case answererAbstained:
		return res.abstain(AnswererAbstains)
	case res.Verified == 0:
		return res.abstain(NoVerifiedClaims)
	case float64(res.Verified) < pol.MinVerifiedFraction*float64(len(res.Claims)):
		return res.abstain(TooManyStruck)
	}
	return res
}

func (r *Result) abstain(reason AbstainReason) *Result {
	r.Abstained = true
	r.Reason = reason
	return r
}
