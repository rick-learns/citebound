# 0011. Fall back to extractive answers when a daily token budget is used up

- Status: Accepted
- Date: 2026-11-17

## Context

Decision 0010 bounds how fast questions arrive and how many are answered at
once, but not how much a live model costs in a day. At the default rate, a
few clients asking all day can still run up a large bill on a paid model, or
use up a free model's daily quota upstream.

Each answer now reports the model tokens it used (`inputTokens` and
`outputTokens` in `done`). Not every call reports them, though. A call that
fails, returns no text, or stops at the token limit with only hidden
reasoning may still be billed, and its counts are lost on the way back. A
provider could also omit counts or send nonsense, such as negative numbers.

## Decision

`citebound serve -daily-tokens N` (default 1,000,000) caps the model tokens
answers use each UTC day, across all clients. `0` disables the cap, and it
has no effect in extractive mode.

- **Fall back, do not refuse.** Once the day's tokens reach the limit,
  questions are answered by the extractive answerer until midnight UTC. The
  demo keeps working, with weaker answers, and every quote is still checked.
  `done` reports `"mode": "extractive"` and `"budgetExhausted": true`, and
  the web summary says why. The `budget_exhausted` error code, reserved until
  now, is dropped: no request is refused for budget.
- **Reserve, then settle.** Each model call is charged 10,000 tokens when it
  starts. That is more than any answer uses: the 4,096-token reply cap plus
  the largest prompt recorded, 4,894 tokens. When the call returns usable
  counts, the charge is replaced by them. Otherwise the reservation stays:
  - the call failed, whatever the reason;
  - the reply had no counts, or a negative one.

  A replay miss is the exception: no model was called, so it is charged
  nothing.
- **Running answers finish.** The check is made when an answer starts, and
  calls in flight count through their reservations. So a day overshoots only
  by the last answers that started under the limit: at most one answer per
  in-flight slot, or about 80,000 tokens at the default cap of 8.
- The first time each day the limit is reached, the server logs a warning.
- Counts saturate instead of overflowing, so a huge reported count uses up
  the day rather than wrapping around.

## Consequences

- A deployment's worst-case spend is about one day's limit, plus the
  overshoot, per day.
- A failed call is charged as a large one, so a run of upstream errors uses
  up the budget early. That errs on the side of spending less.
- State is in memory, per process, like decision 0010: a restart resets the
  day's count, and several replicas would each allow the full limit.
- The budget counts tokens, not money. `costUsd` in `done` is still 0;
  pricing per model would turn the limit into dollars, and is left for
  later.

## Alternatives considered

- **Refuse with `budget_exhausted`.** Simpler, but the demo would stop
  answering for the rest of the day. Falling back costs nothing, since the
  extractive answerer needs no model.
- **Count only reported tokens.** Calls that fail or lose their counts would
  be free, so the breaker could be bypassed by exactly the replies most
  likely to be billed without use.
- **Fix every provider path to keep counts on error.** Worth doing for
  accuracy, but it touches each provider and the recorder, and still leaves
  calls that fail before a reply arrives. The reservation covers all of them.
- **A dollar limit.** Needs a price table per model and provider, kept
  current. Tokens are what the providers report.
