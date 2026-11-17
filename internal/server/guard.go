package server

import (
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

// limiter is a token bucket per client: each holds up to burst tokens and
// refills at rate tokens per second. Each ask takes one.
type limiter struct {
	rate, burst float64
	now         func() time.Time

	mu      sync.Mutex
	clients map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newLimiter(perMinute float64, burst int, now func() time.Time) *limiter {
	return &limiter{rate: perMinute / 60, burst: float64(burst), now: now, clients: map[string]*bucket{}}
}

// allow takes a token for client. If none is left, it reports how long until
// one will be.
func (l *limiter) allow(client string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b := l.clients[client]
	if b == nil {
		if len(l.clients) >= maxClients {
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

// wait is how long a bucket holding tokens takes to reach one.
func (l *limiter) wait(tokens float64) time.Duration {
	return time.Duration((1 - tokens) / l.rate * float64(time.Second))
}

// sweep forgets clients whose buckets have refilled: a new bucket would be
// the same.
func (l *limiter) sweep(now time.Time) {
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

// retryAfter formats d as whole seconds for a Retry-After header, at least 1.
func retryAfter(d time.Duration) int {
	return max(1, int(math.Ceil(d.Seconds())))
}
