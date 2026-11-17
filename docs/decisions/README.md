# Decision records

Every decision that is expensive to reverse, or that a new contributor would
otherwise have to rediscover, is written down here. Records are short,
numbered, and never deleted. A decision that changes gets a new record that
supersedes the old one.

| # | Decision | Status |
| --- | --- | --- |
| [0001](0001-record-decisions.md) | Record decisions in `docs/decisions` | Accepted |
| [0002](0002-go-backend-typescript-ui.md) | Go backend, TypeScript UI, one SQLite file | Accepted |
| [0003](0003-review-and-merge-gate.md) | Small PRs, independent review, and a squash-merge gate | Accepted |
| [0004](0004-pinned-rfc-corpus.md) | Answer only from a pinned corpus of 21 RFCs | Accepted |
| [0005](0005-exact-quote-verification.md) | Verify citations by exact quote match, per paragraph | Accepted |
| [0006](0006-abstention-policy.md) | Abstain unless most claims verify | Accepted |
| [0007](0007-model-providers-and-replay.md) | Call models over plain HTTP, and replay recordings in CI | Accepted |
| [0008](0008-escaped-double-quotes.md) | Read an escaped double quote in a quote as a double quote | Accepted |
| [0009](0009-embed-the-web-ui.md) | Embed the built web UI behind a build tag | Accepted |
| [0010](0010-rate-limit-and-cap-asks.md) | Rate-limit each client and cap concurrent asks | Accepted |
| [0011](0011-daily-token-budget.md) | Fall back to extractive answers when a daily token budget is used up | Accepted |

## Writing a record

1. Copy [`template.md`](template.md) to `NNNN-short-title.md` with the next number.
2. Fill in every section. "Alternatives considered" is not optional.
3. Add a row to the table above in the same pull request as the change it
   describes.
4. To change a decision, add a new record, set the old one's status to
   `Superseded by NNNN`, and update the table.
