package server

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/index"
)

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := newLimiter(60, 2, func() time.Time { return now }) // one a second, bursts of 2
	for i := range 2 {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("ask %d refused within the burst", i)
		}
	}
	if ok, wait := l.allow("a"); ok || wait != 1 {
		t.Fatalf("third ask: ok %v, wait %v; want refused for 1s", ok, wait)
	}
	if ok, _ := l.allow("b"); !ok {
		t.Fatal("another client was refused")
	}
	now = now.Add(500 * time.Millisecond)
	if ok, wait := l.allow("a"); ok || wait != 0.5 {
		t.Fatalf("after 0.5s: ok %v, wait %v; want refused for 0.5s", ok, wait)
	}
	now = now.Add(500 * time.Millisecond)
	if ok, _ := l.allow("a"); !ok {
		t.Fatal("after 1s: refused, want a refilled token")
	}
	now = now.Add(time.Hour)
	for i := range 2 {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("after an hour, ask %d refused: the bucket should hold the burst, no more", i)
		}
	}
	if ok, _ := l.allow("a"); ok {
		t.Fatal("after an hour, a third ask was allowed: refill must stop at the burst")
	}
}

func TestLimiterBoundsClients(t *testing.T) {
	now := time.Unix(0, 0)
	l := newLimiter(60, 1, func() time.Time { return now })
	for i := range maxClients {
		l.allow(strconv.Itoa(i))
	}
	if ok, _ := l.allow("new"); ok {
		t.Fatal("a new client was admitted past maxClients while every bucket was in use")
	}
	now = now.Add(time.Second) // every bucket refills
	l.allow("0")               // and one is in use again
	if ok, _ := l.allow("new"); !ok {
		t.Fatal("a new client was refused after idle buckets refilled")
	}
	if _, kept := l.clients["0"]; !kept || len(l.clients) != 2 {
		t.Errorf("after the sweep: %d clients, client 0 kept %v; want 2, true", len(l.clients), kept)
	}
}

// A full map of busy clients is swept at most once per sweepEvery, however
// many new clients ask: each is refused in constant time.
func TestLimiterSweepsAFullMapRarely(t *testing.T) {
	now := time.Unix(0, 0)
	l := newLimiter(1, 1, func() time.Time { return now }) // a token a minute: buckets stay busy
	for i := range maxClients {
		l.allow(strconv.Itoa(i))
	}
	for i := range 1000 {
		if ok, _ := l.allow("new" + strconv.Itoa(i)); ok {
			t.Fatal("a new client was admitted to a full map of busy clients")
		}
	}
	if l.sweeps != 1 {
		t.Fatalf("1,000 new clients at one instant: %d sweeps, want 1", l.sweeps)
	}
	now = now.Add(sweepEvery / 2)
	l.allow("new")
	if l.sweeps != 1 {
		t.Fatalf("within sweepEvery: %d sweeps, want 1", l.sweeps)
	}
	now = now.Add(sweepEvery / 2)
	l.allow("new")
	if l.sweeps != 2 {
		t.Fatalf("after sweepEvery: %d sweeps, want 2", l.sweeps)
	}
}

func BenchmarkLimiterFullMapNewClient(b *testing.B) {
	now := time.Unix(0, 0)
	l := newLimiter(1, 1, func() time.Time { return now })
	for i := range maxClients {
		l.allow(strconv.Itoa(i))
	}
	for b.Loop() {
		l.allow("new")
	}
}

func TestRetryAfter(t *testing.T) {
	for secs, want := range map[float64]int{0: 1, 0.2: 1, 1: 1, 1.01: 2, 60: 60, 86400: 86400} {
		if got := retryAfter(secs); got != want {
			t.Errorf("retryAfter(%v) = %d, want %d", secs, got, want)
		}
	}
	// At the slowest allowed rate, an empty bucket waits a day.
	l := newLimiter(minAskRate, 1, time.Now)
	l.allow("a")
	if _, wait := l.allow("a"); retryAfter(wait) != 86400 {
		t.Errorf("slowest rate: Retry-After %d, want 86400", retryAfter(wait))
	}
}

func TestGuardSettings(t *testing.T) {
	for _, c := range []struct {
		cfg Config
		ok  bool
	}{
		{Config{}, true},
		{Config{AskRate: 10, AskBurst: 5, MaxInFlight: 8}, true},
		{Config{AskRate: minAskRate}, true},
		{Config{AskRate: 1e9}, true},
		{Config{AskRate: math.NaN()}, false},
		{Config{AskRate: math.Inf(1)}, false},
		{Config{AskRate: math.Inf(-1)}, false},
		{Config{AskRate: -1}, false},
		{Config{AskRate: minAskRate / 2}, false},
		{Config{AskRate: 1e-9}, false},
		{Config{AskBurst: -1}, false},
		{Config{AskRate: 1, AskBurst: maxAskBurst}, true},
		{Config{AskRate: 1, AskBurst: maxAskBurst + 1}, false},
		{Config{AskRate: 1, AskBurst: math.MaxInt}, false},
		{Config{MaxInFlight: -1}, false},
	} {
		c.cfg.Pipeline = &answer.Pipeline{}
		if _, err := New(c.cfg); (err == nil) != c.ok {
			t.Errorf("New(rate %v, burst %d, in flight %d): err %v, want ok %v", c.cfg.AskRate, c.cfg.AskBurst, c.cfg.MaxInFlight, err, c.ok)
		}
	}
}

func TestClientKey(t *testing.T) {
	for _, c := range []struct {
		remote, header, value, want string
	}{
		{"203.0.113.5:1234", "", "", "203.0.113.5"},
		{"[::ffff:203.0.113.5]:1234", "", "", "203.0.113.5"},
		{"[2001:db8:1:2:3:4:5:6]:80", "", "", "2001:db8:1:2::/64"},
		{"[2001:db8:1:2:ffff::1]:80", "", "", "2001:db8:1:2::/64"},
		{"[fe80::1%eth0]:80", "", "", "fe80::/64"},
		// A header counts only when the server is told a proxy sets it.
		{"10.0.0.1:80", "", "198.51.100.7", "10.0.0.1"},
		{"10.0.0.1:80", "Fly-Client-IP", "198.51.100.7", "198.51.100.7"},
		{"10.0.0.1:80", "Fly-Client-IP", " 2001:db8::9 ", "2001:db8::/64"},
		{"10.0.0.1:80", "Fly-Client-IP", "", "10.0.0.1"},
		{"10.0.0.1:80", "Fly-Client-IP", "not-an-ip", "invalid"},
		{"10.0.0.1:80", "Fly-Client-IP", "198.51.100.7, 10.0.0.2", "invalid"},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/ask", nil)
		r.RemoteAddr = c.remote
		if c.value != "" {
			r.Header.Set("Fly-Client-IP", c.value)
		}
		if got := clientKey(r, c.header); got != c.want {
			t.Errorf("clientKey(%s, %s=%q) = %q, want %q", c.remote, c.header, c.value, got, c.want)
		}
	}
}

type guardReply struct {
	status     int
	retryAfter string
	code       string
}

func post(t *testing.T, ts *httptest.Server, body string) guardReply {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/ask", "application/json", strings.NewReader(body)) //nolint:noctx // test
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	r := guardReply{status: resp.StatusCode, retryAfter: resp.Header.Get("Retry-After")}
	if resp.StatusCode != http.StatusOK {
		var e struct{ Error errorBody }
		if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
			t.Fatal(err)
		}
		r.code = e.Error.Code
	}
	return r
}

func TestAskRateLimit(t *testing.T) {
	ts := newServerWith(t, answer.Extractive{}, func(c *Config) { c.AskRate, c.AskBurst = 1, 1 })
	const q = `{"question":"refresh tokens"}`
	if r := post(t, ts, `{"question":""}`); r.status != http.StatusBadRequest {
		t.Fatalf("empty question: status %d", r.status)
	}
	if r := post(t, ts, q); r.status != http.StatusOK {
		t.Fatalf("first ask: status %d; a bad request must not use up the token", r.status)
	}
	if r := post(t, ts, q); r.status != http.StatusTooManyRequests || r.code != "rate_limited" || r.retryAfter != "60" {
		t.Errorf("second ask = %+v, want 429 rate_limited, Retry-After 60", r)
	}
}

// gate answers only when released, signalling when it starts.
type gate struct{ started, release chan struct{} }

func (g gate) Answer(ctx context.Context, q string, p []index.Hit) (string, error) {
	g.started <- struct{}{}
	<-g.release
	return answer.Extractive{}.Answer(ctx, q, p)
}

func TestAskInFlightCap(t *testing.T) {
	g := gate{started: make(chan struct{}), release: make(chan struct{})}
	ts := newServerWith(t, g, func(c *Config) { c.MaxInFlight = 1 })
	const q = `{"question":"refresh tokens"}`
	first := make(chan int, 1) // its status; t.Fatal must not run off the test goroutine
	go func() {
		resp, err := http.Post(ts.URL+"/api/ask", "application/json", strings.NewReader(q)) //nolint:noctx // test
		if err != nil {
			first <- 0
			return
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		first <- resp.StatusCode
	}()
	<-g.started
	if r := post(t, ts, q); r.status != http.StatusServiceUnavailable || r.code != "unavailable" || r.retryAfter != "5" {
		t.Errorf("ask while one is answering = %+v, want 503 unavailable, Retry-After 5", r)
	}
	close(g.release)
	if status := <-first; status != http.StatusOK {
		t.Fatalf("first ask: status %d", status)
	}
	go func() { <-g.started }()
	if r := post(t, ts, q); r.status != http.StatusOK {
		t.Errorf("ask after the first finished: status %d; its slot was not released", r.status)
	}
}
