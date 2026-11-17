package server

import (
	"math"
	"sync"
	"time"
)

// budget counts the model tokens answers use each UTC day, so the server can
// stop calling the model once a day's limit is reached (decision 0011).
type budget struct {
	limit int
	now   func() time.Time

	mu    sync.Mutex
	day   int64 // days since the Unix epoch, UTC
	spent int
}

// exhausted reports whether today's tokens have reached the limit.
func (b *budget) exhausted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roll()
	return b.spent >= b.limit
}

// spend adds an answer's tokens to today's count. It reports whether they
// used up the budget, so the server can log that once a day. Negative counts
// are ignored, and the count saturates rather than overflow.
func (b *budget) spend(tokens int) (usedUp bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roll()
	before := b.spent
	if tokens > 0 {
		b.spent += min(tokens, math.MaxInt-b.spent)
	}
	return before < b.limit && b.spent >= b.limit
}

// roll starts a new count when the UTC day changes.
func (b *budget) roll() {
	if d := b.now().Unix() / (24 * 60 * 60); d != b.day {
		b.day, b.spent = d, 0
	}
}
