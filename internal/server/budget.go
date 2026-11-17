package server

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/index"
	"github.com/rick-learns/citebound/internal/llm"
)

// modelAnswerer is an answerer that can say what it will send a model, so a
// call can be charged an upper bound on its tokens before it starts.
// answer.Model is one.
type modelAnswerer interface {
	answer.Answerer
	Request(question string, passages []index.Hit) llm.Request
}

// framingTokens covers what a provider adds around the prompt: roles and
// message separators.
const framingTokens = 100

// reservation is an upper bound on the tokens req can use (decision 0011).
// No byte-level tokenizer makes a token of less than one byte, so the
// prompt's length in bytes bounds its tokens, and the reply, hidden reasoning
// included, stops at MaxTokens.
func reservation(req llm.Request) int {
	return len(req.System) + len(req.User) + req.MaxTokens + framingTokens
}

// budget counts the model tokens answers use each UTC day, so the server can
// stop calling the model once a day's limit is reached.
type budget struct {
	limit  int
	now    func() time.Time
	usedUp func() // called the first time each day an answer falls back

	mu     sync.Mutex
	day    int64 // days since the Unix epoch, UTC
	spent  int
	warned bool
}

// exhausted reports whether today's tokens have reached the limit, so the
// answer starting now must fall back.
func (b *budget) exhausted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roll()
	if b.spent < b.limit {
		return false
	}
	if !b.warned {
		b.warned = true
		if b.usedUp != nil {
			b.usedUp()
		}
	}
	return true
}

// reserve charges n tokens for a model call starting now, and returns the
// day charged, for settle.
func (b *budget) reserve(n int) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roll()
	b.add(n)
	return b.day
}

// settle replaces a reservation of n tokens made on day with the tokens the
// call used. A call that ran past midnight is charged to the new day.
func (b *budget) settle(day int64, n, used int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roll()
	if day == b.day {
		b.spent = max(0, b.spent-n)
	}
	b.add(used)
}

// add charges n tokens, saturating rather than overflowing.
func (b *budget) add(n int) {
	b.spent += min(max(0, n), math.MaxInt-b.spent)
}

// roll starts a new count when the UTC day changes.
func (b *budget) roll() {
	if d := b.now().Unix() / (24 * 60 * 60); d != b.day {
		b.day, b.spent, b.warned = d, 0, false
	}
}

// budgeted charges each answer from its answerer to a budget.
type budgeted struct {
	modelAnswerer
	budget *budget
}

// Answer implements answer.Answerer.
func (m budgeted) Answer(ctx context.Context, question string, passages []index.Hit) (answer.Reply, error) {
	n := reservation(m.Request(question, passages))
	day := m.budget.reserve(n)
	r, err := m.modelAnswerer.Answer(ctx, question, passages)
	m.budget.settle(day, n, charge(r, err, n))
	return r, err
}

// charge is what a call reserved at n tokens is charged: its reported tokens
// when the reply has usable counts, and otherwise n. An error, or a reply
// with counts missing or negative, may still have been billed. A question
// missing from the recordings never reached a model.
func charge(r answer.Reply, err error, n int) int {
	switch {
	case errors.Is(err, llm.ErrNotRecorded):
		return 0
	case err != nil || r.InputTokens < 0 || r.OutputTokens < 0 || r.InputTokens+r.OutputTokens == 0:
		return n
	}
	return min(r.InputTokens, math.MaxInt-r.OutputTokens) + r.OutputTokens
}
