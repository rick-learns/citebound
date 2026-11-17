# Examples

Answers in the [answer format](../docs/answer-format.md), for `citebound check`,
`make demo`, and [the limitations](../docs/limitations.md). Three are hand-written
and two are real model answers.

| File | Shows |
| --- | --- |
| [`fabricated-quote.txt`](fabricated-quote.txt) | A real quote verifies, and the same sentence altered to say "within 60 seconds" is struck as `quote-not-found` |
| [`false-claim.txt`](false-claim.txt) | A real quote beside a claim that contradicts it. It **passes**: verification proves a quote is real, not that it supports the claim |
| [`paraphrase.txt`](paraphrase.txt) | A true claim with a paraphrased quote. It is struck: only the RFC's own words count |
| [`escaped-figure.txt`](escaped-figure.txt) | A real model answer (Nemotron 3 Ultra) whose real quote is struck, because the model wrote the figure's line breaks as `\\\n` |
| [`cut-off.txt`](cut-off.txt) | A real model answer cut off at the token limit: its last line is struck as `uncited` |

```sh
go run ./cmd/citebound check examples/fabricated-quote.txt   # exits 1
go run ./cmd/citebound check examples/false-claim.txt        # exits 0
```
