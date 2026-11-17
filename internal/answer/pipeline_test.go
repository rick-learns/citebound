package answer

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/verify"
)

type fakeSearcher struct {
	hits []index.Hit
	err  error
}

func (f fakeSearcher) Search(context.Context, string, int) ([]index.Hit, error) {
	return f.hits, f.err
}

type fixedAnswer struct {
	text string
	err  error
}

func (f fixedAnswer) Answer(context.Context, string, []index.Hit) (string, error) {
	return f.text, f.err
}

func TestPipeline(t *testing.T) {
	para := "Refresh tokens MUST be kept confidential in transit and storage."
	lookup := mapLookup{"RFC1 §1 ¶1": para}
	hits := []index.Hit{hit("RFC1 §1 ¶1", para, false)}
	good := `Refresh tokens are secret. [RFC1 §1 ¶1: "MUST be kept confidential in transit"]`
	fake := `Refresh tokens are public. [RFC1 §1 ¶1: "MAY be shared with anyone at all"]`
	uncited := `Refresh tokens are great.`

	tests := []struct {
		name     string
		search   fakeSearcher
		answer   string
		verified int
		claims   int
		reason   AbstainReason // "" means answered
	}{
		{"no passages", fakeSearcher{}, good, 0, 0, NoPassages},
		{"empty query", fakeSearcher{err: index.ErrEmptyQuery}, good, 0, 0, NoPassages},
		{"answerer abstains", fakeSearcher{hits: hits}, verify.AbstainMarker, 0, 0, AnswererAbstains},
		{"answerer abstains after a claim", fakeSearcher{hits: hits}, good + "\n" + verify.AbstainMarker, 1, 1, AnswererAbstains},
		{"all struck", fakeSearcher{hits: hits}, fake + "\n" + uncited, 0, 2, NoVerifiedClaims},
		{"empty answer", fakeSearcher{hits: hits}, "", 0, 0, NoVerifiedClaims},
		{"one of three", fakeSearcher{hits: hits}, strings.Join([]string{good, fake, uncited}, "\n"), 1, 3, TooManyStruck},
		{"one of two", fakeSearcher{hits: hits}, good + "\n" + fake, 1, 2, ""},
		{"all verified", fakeSearcher{hits: hits}, good + "\n" + good, 2, 2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pipeline{Index: tt.search, Lookup: lookup, Answerer: fixedAnswer{text: tt.answer}, Policy: DefaultPolicy}
			res, err := p.Ask(context.Background(), "q", nil)
			if err != nil {
				t.Fatal(err)
			}
			if res.Verified != tt.verified || len(res.Claims) != tt.claims {
				t.Errorf("verified %d of %d, want %d of %d", res.Verified, len(res.Claims), tt.verified, tt.claims)
			}
			if res.Reason != tt.reason || res.Abstained != (tt.reason != "") {
				t.Errorf("abstained %v (%q), want reason %q", res.Abstained, res.Reason, tt.reason)
			}
		})
	}
}

func TestPipelineEvents(t *testing.T) {
	para := "Refresh tokens MUST be kept confidential in transit and storage."
	p := &Pipeline{
		Index:    fakeSearcher{hits: []index.Hit{hit("RFC1 §1 ¶1", para, false)}},
		Lookup:   mapLookup{"RFC1 §1 ¶1": para},
		Answerer: fixedAnswer{text: "A. [RFC1 §1 ¶1: \"MUST be kept confidential in transit\"]\nB."},
		Policy:   DefaultPolicy,
	}
	var got []string
	_, err := p.Ask(context.Background(), "q", func(e Event) {
		switch {
		case e.Passages != nil:
			got = append(got, "passages")
		case e.Claim != nil:
			got = append(got, "claim "+e.Claim.Text)
		case e.Verdict != nil:
			got = append(got, "verdict "+string(e.Verdict.Status))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"passages", "claim A.", "verdict verified", "claim B.", "verdict struck"}
	if !slices.Equal(got, want) {
		t.Errorf("events = %q, want %q", got, want)
	}
}

func TestPipelineEventsWithNoPassages(t *testing.T) {
	for name, search := range map[string]fakeSearcher{
		"no hits":     {},
		"empty query": {err: index.ErrEmptyQuery},
	} {
		p := &Pipeline{Index: search, Answerer: fixedAnswer{}, Policy: DefaultPolicy}
		var events []Event
		res, err := p.Ask(context.Background(), "q", func(e Event) { events = append(events, e) })
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].Passages == nil || len(events[0].Passages) != 0 {
			t.Errorf("%s: events = %+v, want one empty, non-nil passages event", name, events)
		}
		if res.Reason != NoPassages {
			t.Errorf("%s: reason %q", name, res.Reason)
		}
	}
}

func TestPipelineErrors(t *testing.T) {
	boom := errors.New("boom")
	hits := []index.Hit{{Ref: corpus.Ref{RFC: 1, Section: "1", Paragraph: 1}}}
	for name, p := range map[string]*Pipeline{
		"search": {Index: fakeSearcher{err: boom}, Answerer: fixedAnswer{}},
		"answer": {Index: fakeSearcher{hits: hits}, Answerer: fixedAnswer{err: boom}},
	} {
		if _, err := p.Ask(context.Background(), "q", nil); !errors.Is(err, boom) {
			t.Errorf("%s: err = %v, want boom", name, err)
		}
	}
}
