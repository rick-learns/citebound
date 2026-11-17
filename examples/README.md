# Examples

Hand-written answers in the [answer format](../docs/answer-format.md), for
`citebound check` and `make demo`.

| File | Shows |
| --- | --- |
| [`fabricated-quote.txt`](fabricated-quote.txt) | A real quote verifies, and the same sentence altered to say "within 60 seconds" is struck as `quote-not-found` |
| [`false-claim.txt`](false-claim.txt) | A real quote beside a claim that contradicts it. It **passes**: verification proves a quote is real, not that it supports the claim |

```sh
go run ./cmd/citebound check examples/fabricated-quote.txt   # exits 1
go run ./cmd/citebound check examples/false-claim.txt        # exits 0
```
