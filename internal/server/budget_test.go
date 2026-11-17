package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/llm"
)

func TestBudget(t *testing.T) {
	const reserve = 10_000
	now := time.Date(2026, 11, 17, 23, 0, 0, 0, time.UTC)
	warnings := 0
	b := &budget{limit: 15_000, now: func() time.Time { return now }, usedUp: func() { warnings++ }}

	day := b.reserve(reserve)
	if b.exhausted() {
		t.Fatal("exhausted after one reservation of 10,000 of 15,000")
	}
	second := b.reserve(reserve) // 20,000 reserved: in-flight calls count
	if warnings != 0 {
		t.Fatal("warned before any answer fell back")
	}
	for range 2 {
		if !b.exhausted() {
			t.Fatal("two calls in flight: not exhausted")
		}
	}
	if warnings != 1 {
		t.Fatalf("%d warnings after two fallbacks, want 1", warnings)
	}
	b.settle(day, reserve, 3741)
	b.settle(second, reserve, 3741)
	if b.exhausted() || b.spent != 7482 {
		t.Fatalf("settled at 3,741 each: spent %d, exhausted %v; want 7,482, false", b.spent, b.exhausted())
	}
	b.settle(b.reserve(reserve), reserve, reserve) // no usable counts: the reservation stays
	if !b.exhausted() || warnings != 1 {
		t.Fatalf("over the limit again: exhausted %v, %d warnings; want true, still 1 that day", b.exhausted(), warnings)
	}

	// A call that runs past midnight is charged to the new day, without
	// releasing a reservation it never made there.
	late := b.reserve(reserve)
	now = now.Add(2 * time.Hour)
	b.reserve(reserve) // a call started after midnight
	b.settle(late, reserve, 3741)
	if b.spent != reserve+3741 {
		t.Errorf("after midnight: spent %d, want %d", b.spent, reserve+3741)
	}
	b.spent = 0
	b.settle(b.reserve(reserve), reserve, math.MaxInt)
	if !b.exhausted() || b.spent != math.MaxInt || warnings != 2 {
		t.Errorf("huge count: spent %d, %d warnings; want MaxInt, and a warning for the new day", b.spent, warnings)
	}
}

func TestCharge(t *testing.T) {
	for _, c := range []struct {
		name string
		r    answer.Reply
		err  error
		want int
	}{
		{"reported", answer.Reply{InputTokens: 1631, OutputTokens: 2110}, nil, 3741},
		{"output only", answer.Reply{OutputTokens: 7}, nil, 7},
		{"no counts", answer.Reply{Text: "A."}, nil, 12_345},
		{"negative input", answer.Reply{InputTokens: -1, OutputTokens: 2110}, nil, 12_345},
		{"negative output", answer.Reply{InputTokens: 1631, OutputTokens: -5000}, nil, 12_345},
		{"error", answer.Reply{}, llm.ErrEmpty, 12_345},
		{"error with counts", answer.Reply{InputTokens: 1, OutputTokens: 1}, errors.New("recording failed"), 12_345},
		{"not recorded", answer.Reply{}, fmt.Errorf("replay: %w", llm.ErrNotRecorded), 0},
		{"huge", answer.Reply{InputTokens: math.MaxInt, OutputTokens: math.MaxInt}, nil, math.MaxInt},
	} {
		if got := charge(c.r, c.err, 12_345); got != c.want {
			t.Errorf("%s: charge = %d, want %d", c.name, got, c.want)
		}
	}
}

type failingProvider struct{}

func (failingProvider) Name() string { return "test/failing" }

func (failingProvider) Complete(context.Context, llm.Request) (*llm.Response, error) {
	return nil, llm.ErrEmpty // billed, but with no counts
}

// A billed call that loses its counts is charged an upper bound on its
// tokens, however large the passages it was sent: the prompt's bytes, plus
// the reply cap.
func TestBudgetChargesFailedCallsAnUpperBound(t *testing.T) {
	figure := strings.Repeat("0123456789 ", 2000) // 22,000 bytes
	var hits []index.Hit
	for i := range 8 {
		hits = append(hits, index.Hit{Ref: corpus.Ref{RFC: 1, Section: "1", Paragraph: i + 1}, Text: figure, Figure: true})
	}
	question := strings.Repeat("9 ", 500)
	b := &budget{limit: math.MaxInt, now: time.Now}
	m := budgeted{answer.Model{Provider: failingProvider{}}, b}
	if _, err := m.Answer(context.Background(), question, hits); !errors.Is(err, llm.ErrEmpty) {
		t.Fatalf("err = %v, want ErrEmpty", err)
	}
	req := answer.Model{}.Request(question, hits)
	if min := len(req.System) + len(req.User) + 4096; b.spent < min || b.spent != reservation(req) {
		t.Errorf("charged %d, want the reservation %d, at least the prompt's %d bytes plus the 4,096-token reply cap", b.spent, reservation(req), min)
	}
	if b.spent < 8*len(figure) {
		t.Errorf("charged %d, less than the figures' %d bytes", b.spent, 8*len(figure))
	}
}

// Normalization can make a prompt longer than its bytes: under NFC, U+0FB9
// (3 bytes) becomes U+0F90 U+0FB5 (6). A question of 995 of them must be
// bounded by its normalized length.
func TestPromptBoundCoversNormalization(t *testing.T) {
	for _, c := range []struct {
		s    string
		want int
	}{
		{"plain ASCII", 11},
		{"\u0fb9", 6},       // NFC decomposes it
		{"\ufdfa", 33},      // NFKC: 18 Arabic letters and spaces
		{"\ufb01", 3},       // NFKC shortens the ligature to "fi"; the raw bytes are longer
		{"e\u0301", 3},      // NFC composes to é (2 bytes); the raw form is longer
		{"\u0fb9\ufb01", 9}, // NFC 9 bytes, NFKC 8, raw 6
	} {
		if got := promptBound(c.s); got != c.want {
			t.Errorf("promptBound(%+q) = %d, want %d", c.s, got, c.want)
		}
	}
	question := "PRNG " + strings.Repeat("\u0fb9", 995)
	hits := []index.Hit{{Ref: corpus.Ref{RFC: 1, Section: "1", Paragraph: 1}, Text: "A pseudo-random number generator."}}
	req := answer.Model{}.Request(question, hits)
	if got, min := reservation(req), len(req.System)+len(req.User)+995*3+req.MaxTokens; got < min {
		t.Errorf("reservation = %d, want at least %d: the raw bytes plus 3 for each expanded character", got, min)
	}
}

func TestBudgetNeedsAModelAnswerer(t *testing.T) {
	for _, c := range []struct {
		mode string
		a    answer.Answerer
		ok   bool
	}{
		{"model", answer.Model{}, true},
		{"replay", metered{}, true},
		{"model", fixed(""), false},
		{"extractive", answer.Extractive{}, true}, // no budget in extractive mode
	} {
		_, err := New(Config{Mode: c.mode, DailyTokens: 1000, Pipeline: &answer.Pipeline{Answerer: c.a}})
		if (err == nil) != c.ok {
			t.Errorf("%s mode, %T: err %v, want ok %v", c.mode, c.a, err, c.ok)
		}
	}
}

// Each metered answer uses 3,741 tokens, so a budget of 5,000 allows two.
func TestAskFallsBackOverBudget(t *testing.T) {
	ts := newServerWith(t, metered{}, func(c *Config) { c.Mode, c.Model, c.DailyTokens = "model", "test/m", 5000 })
	const q = `{"question":"Can a refresh token be bound to a client?"}`
	for i, want := range []struct {
		mode, model string
		tokens      float64
		exhausted   bool
	}{
		{"model", "test/m", 1631, false},
		{"model", "test/m", 1631, false},
		{"extractive", "", 0, true},
		{"extractive", "", 0, true},
	} {
		_, events := ask(t, ts, q)
		d := events[len(events)-1]
		if d.name != "done" || d.data["mode"] != want.mode || d.data["model"] != want.model ||
			d.data["inputTokens"] != want.tokens || d.data["budgetExhausted"] != want.exhausted {
			t.Errorf("ask %d: %s %v; want mode %q, model %q, %v input tokens, budgetExhausted %v", i+1, d.name, d.data, want.mode, want.model, want.tokens, want.exhausted)
		}
		if d.data["verified"] == 0.0 {
			t.Errorf("ask %d: nothing verified, want an answer either way", i+1)
		}
	}
}

func TestBudgetIgnoredWhenExtractive(t *testing.T) {
	ts := newServerWith(t, answer.Extractive{}, func(c *Config) { c.DailyTokens = 1 })
	for i := range 3 {
		_, events := ask(t, ts, `{"question":"Can a refresh token be bound to a client?"}`)
		if d := events[len(events)-1]; d.data["budgetExhausted"] != false {
			t.Errorf("ask %d: budgetExhausted %v, want false in extractive mode", i+1, d.data["budgetExhausted"])
		}
	}
}
