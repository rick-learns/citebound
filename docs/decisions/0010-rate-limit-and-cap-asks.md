# 0010. Rate-limit each client and cap concurrent asks

- Status: Accepted
- Date: 2026-11-17

## Context

A public demo of `citebound serve` with a live model spends money and
upstream rate limit on every question. One client in a loop, or many at once,
could use up both and leave the demo unusable for everyone else. Free models
are also rate-limited upstream, so a burst that gets past us fails there
anyway, more slowly.

Some limits already exist: a question is at most 1,000 characters, a request
body at most 16 KiB, a model reply at most 4,096 tokens (decision 0007), and
an answer at most 60 seconds. What is missing is a limit on how often, and how
many at once.

## Decision

`POST /api/ask` gets two guards, run after the request is validated, so a
malformed request costs nothing:

- **Per-client rate limit.** Each client has a token bucket: `-ask-rate`
  questions a minute (default 10), in bursts of up to `-ask-burst` (default
  5). An empty bucket gets `429 rate_limited` with `Retry-After` in whole
  seconds.
  - A client is its IP address. An IPv6 client is its /64, since one
    subscriber usually holds a whole /64 and could otherwise rotate through
    it.
  - Behind a reverse proxy, every connection comes from the proxy.
    `-client-ip-header` names a header the proxy sets to the client's address
    (for example `Fly-Client-IP`). It must hold one address; anything else
    shares a single bucket. The header is ignored unless named, because a
    client could otherwise choose its own key.
  - At most 10,000 clients are tracked. When the map is full, a new client
    triggers a sweep that drops buckets that have refilled, since they equal
    a new one. If none has, the new client is refused. Sweeps run at most
    once a second, and a client refused between them costs a map lookup, so
    a stream of new addresses cannot make every request scan the map.
  - The tradeoff: an attacker holding more than 10,000 addresses (IPv4, or
    IPv6 /64s), each asking often enough to stay busy, locks out new
    visitors while it lasts. Clients already tracked are unaffected, and the
    in-flight cap still bounds what the attack costs. Failing open instead
    would let the same attacker bypass the rate limit entirely.
- **Global concurrency cap.** At most `-max-inflight` questions (default 8)
  are answered at once. Past that, `503 unavailable` with `Retry-After: 5`.
  The request is refused immediately, not queued: a queue would hold
  connections open and hide the overload from the client.

`0` disables either guard. The defaults are on, at levels one person at a
local server will not reach. `server.New`, and so `citebound serve`, refuses
settings that would silently disable a guard or overflow it: a rate that is
negative, not finite, or slower than one ask a day, and a negative burst or
in-flight cap.

## Consequences

- One client cannot use more than its share of the model, and the server
  never runs more model calls than the cap.
- The web UI maps both codes to friendly messages and shows the server's
  message, which names the wait.
- State is in memory, per process. Several replicas would each allow the full
  rate. That is fine for one small demo instance, and would need a shared
  store to change.
- Clients behind one NAT share a bucket. At 10 a minute this is unlikely to
  matter for a demo.
- A daily cost limit is separate: the rate limit bounds speed, not total
  spend. It is planned with token accounting in `done` (issue #14).

## Alternatives considered

- **`golang.org/x/time/rate`.** A good limiter, but a new dependency for about
  60 lines, and it does not bound the per-client map, which is the part that
  needs care.
- **Rate-limit every endpoint.** `/api/meta` and `/api/section` are cheap,
  read-only, and served from memory. Limiting them would slow the citation
  panel for no saving.
- **Queue asks over the cap.** Rejected above: it holds connections and turns
  overload into timeouts.
- **Trust `X-Forwarded-For`.** It is a list that clients can prepend to, and
  which entry is real depends on the number of proxies. One header that names
  one address, set by a named proxy, is harder to get wrong.
