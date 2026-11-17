package server

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/llm"
)

func TestBudget(t *testing.T) {
	now := time.Date(2026, 11, 17, 23, 0, 0, 0, time.UTC)
	warnings := 0
	b := &budget{limit: 15_000, now: func() time.Time { return now }, usedUp: func() { warnings++ }}

	day := b.reserve()
	if b.exhausted() {
		t.Fatal("exhausted after one reservation of 10,000 of 15,000")
	}
	second := b.reserve() // 20,000 reserved: in-flight calls count
	if warnings != 0 {
		t.Fatal("warned before any answer fell back")
	}
	if !b.exhausted() || !b.exhausted() || warnings != 1 {
		t.Fatalf("two calls in flight: %d warnings after two fallbacks; want exhausted, 1", warnings)
	}
	b.settle(day, 3741)
	b.settle(second, 3741)
	if b.exhausted() || b.spent != 7482 {
		t.Fatalf("settled at 3,741 each: spent %d, exhausted %v; want 7,482, false", b.spent, b.exhausted())
	}
	b.settle(b.reserve(), reserveTokens) // no usable counts: the reservation stays
	if !b.exhausted() || warnings != 1 {
		t.Fatalf("over the limit again: exhausted %v, %d warnings; want true, still 1 that day", b.exhausted(), warnings)
	}

	// A call that runs past midnight is charged to the new day, without
	// releasing a reservation it never made there.
	late := b.reserve()
	now = now.Add(2 * time.Hour)
	b.reserve() // a call started after midnight
	b.settle(late, 3741)
	if b.spent != reserveTokens+3741 {
		t.Errorf("after midnight: spent %d, want %d", b.spent, reserveTokens+3741)
	}
	b.spent = 0
	b.settle(b.reserve(), math.MaxInt)
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
		{"no counts", answer.Reply{Text: "A."}, nil, reserveTokens},
		{"negative input", answer.Reply{InputTokens: -1, OutputTokens: 2110}, nil, reserveTokens},
		{"negative output", answer.Reply{InputTokens: 1631, OutputTokens: -5000}, nil, reserveTokens},
		{"error", answer.Reply{}, llm.ErrEmpty, reserveTokens},
		{"error with counts", answer.Reply{InputTokens: 1, OutputTokens: 1}, errors.New("recording failed"), reserveTokens},
		{"not recorded", answer.Reply{}, fmt.Errorf("replay: %w", llm.ErrNotRecorded), 0},
		{"huge", answer.Reply{InputTokens: math.MaxInt, OutputTokens: math.MaxInt}, nil, math.MaxInt},
	} {
		if got := charge(c.r, c.err); got != c.want {
			t.Errorf("%s: charge = %d, want %d", c.name, got, c.want)
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
