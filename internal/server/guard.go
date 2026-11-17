package server

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// maxClients bounds the rate limiter's memory. Past it, clients not yet seen
// are refused until idle ones age out (decision 0010).
const maxClients = 10000

// sweepEvery bounds how often a full client map is swept. Between sweeps, a
// client not yet seen is refused without one, so a stream of new clients
// costs one sweep a second, not one each.
const sweepEvery = time.Second

// minAskRate is the slowest supported rate: one ask a day.
const minAskRate = 1.0 / (24 * 60)

// maxAskBurst is the largest supported burst. A larger one is no limit, and
// near 2^53 the float bucket stops changing when a token is taken.
const maxAskBurst = 1_000_000

// validateGuards rejects settings that would silently disable a guard or
// overflow its arithmetic. Zero disables a guard; nothing else does.
func (c Config) validateGuards() error {
	switch {
	case math.IsNaN(c.AskRate) || math.IsInf(c.AskRate, 0) || c.AskRate < 0:
		return fmt.Errorf("server: ask rate %v: want a finite number of asks a minute, or 0 to disable", c.AskRate)
	case c.AskRate > 0 && c.AskRate < minAskRate:
		return fmt.Errorf("server: ask rate %v: want at least one ask a day (%.6f a minute), or 0 to disable", c.AskRate, minAskRate)
	case c.AskBurst < 0 || c.AskBurst > maxAskBurst:
		return fmt.Errorf("server: ask burst %d: want 0 to %d", c.AskBurst, maxAskBurst)
	case c.MaxInFlight < 0:
		return errors.New("server: max in flight must not be negative, or 0 to disable")
	}
	return nil
}

// limiter is a token bucket per client: each holds up to burst tokens and
// refills at rate tokens per second. Each ask takes one.
type limiter struct {
	rate, burst float64
	now         func() time.Time

	mu        sync.Mutex
	clients   map[string]*bucket
	lastSweep time.Time
	sweeps    int // for tests
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(perMinute float64, burst int, now func() time.Time) *limiter {
	return &limiter{rate: perMinute / 60, burst: float64(burst), now: now, clients: map[string]*bucket{}}
}

// allow takes a token for client. If none is left, it reports how many
// seconds until one will be.
func (l *limiter) allow(client string) (bool, float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b := l.clients[client]
	if b == nil {
		if len(l.clients) >= maxClients && now.Sub(l.lastSweep) >= sweepEvery {
			l.sweep(now)
		}
		if len(l.clients) >= maxClients {
			return false, l.wait(0)
		}
		b = &bucket{tokens: l.burst, last: now}
		l.clients[client] = b
	}
	b.tokens = l.refill(b, now)
	b.last = now
	if b.tokens < 1 {
		return false, l.wait(b.tokens)
	}
	b.tokens--
	return true, 0
}

func (l *limiter) refill(b *bucket, now time.Time) float64 {
	return math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
}

// wait is how many seconds a bucket holding tokens takes to reach one. Seconds
// as a float, not a Duration: at the slowest rates the nanoseconds overflow.
func (l *limiter) wait(tokens float64) float64 {
	return (1 - tokens) / l.rate
}

// sweep forgets clients whose buckets have refilled: a new bucket would be
// the same.
func (l *limiter) sweep(now time.Time) {
	l.lastSweep = now
	l.sweeps++
	for k, b := range l.clients {
		if l.refill(b, now) >= l.burst {
			delete(l.clients, k)
		}
	}
}

// clientKey identifies the client to rate-limit: the address in header, when
// a trusted proxy sets it, else the connection's. IPv6 clients are keyed by
// their /64, which one subscriber usually holds whole.
func clientKey(r *http.Request, header string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if header != "" {
		if v := strings.TrimSpace(r.Header.Get(header)); v != "" {
			host = v
		}
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		// Unparsable: share one bucket rather than let the text pick a fresh one.
		return "invalid"
	}
	addr = addr.Unmap()
	if addr.Is6() {
		return netip.PrefixFrom(addr.WithZone(""), 64).Masked().String()
	}
	return addr.String()
}

// retryAfter rounds secs up to whole seconds for a Retry-After header, at
// least 1. validateGuards keeps it under a day.
func retryAfter(secs float64) int {
	return max(1, int(math.Ceil(secs)))
}
