# 0006. Abstain unless most claims verify

- Status: Accepted
- Date: 2026-11-17

## Context

Verification ([0005](0005-exact-quote-verification.md)) strikes claims whose
quotes are not in the cited paragraph. That leaves a second question: when
some claims are struck, is what remains still an answer?

Two failure modes pull in opposite directions. Answering too eagerly shows a
user a confident answer built on a model that was mostly making things up.
Abstaining too eagerly makes the assistant useless. Both are measured by the
unanswerable eval set (abstention precision and recall), so the policy must
be simple enough to tune against those numbers.

## Decision

The pipeline withholds an answer, and says so, in the first case that
applies:

| Reason | When |
| --- | --- |
| `no-passages` | Retrieval found nothing, for example a question of only stopwords. |
| `answerer-abstained` | The answerer wrote `INSUFFICIENT_EVIDENCE` anywhere in its answer. |
| `no-verified-claims` | No claim verified, including an empty answer. |
| `too-many-struck` | Fewer than half of the claims verified. |

Otherwise the answer is shown with every claim: verified claims with their
quotes, struck claims struck through with the reason.

- **Half, with ties answering.** One verified claim of two is an answer; one
  of three is not. An answer that is mostly fabricated is not trusted, even
  for the claims that happen to check out, because the surviving claims are
  likely cherry-picked from the same wrong frame.
- **The abstain marker wins.** If an answerer says the evidence is
  insufficient, we believe it, even if it also wrote claims.
- **Withheld claims stay visible.** When the pipeline abstains, the claims
  and their verdicts are still returned and shown as withheld, so a reader
  can see why.

## Consequences

- The threshold is one number, `answer.DefaultPolicy.MinVerifiedFraction`,
  that the eval scorecard can sweep.
- Uncited lines count as struck claims. Model prompts must forbid preambles
  such as "Here is the answer:", or every answer loses a claim's worth of
  margin.
- The extractive answerer quotes verbatim, so it never trips the fraction
  rule; it abstains only by writing the marker.

## Alternatives considered

- **Show whatever verified.** Simplest, but rewards an answerer for padding
  with guesses, since the fabricated ones are silently dropped.
- **Abstain if anything is struck.** Safe, but one formatting slip by a model
  would hide an otherwise correct answer.
- **Retrieval score threshold.** BM25 scores are not comparable across
  queries, so a fixed cutoff would be arbitrary. The answerer, which reads
  the passages, is better placed to decide relevance.
- **A model judge of relevance.** Adds cost and a second fallible model to
  every question; it belongs in the M6 critic experiment, measured, not in
  the default path.
