# Evals

Labeled questions that CI scores on every pull request. Scores are
deterministic: the corpus is pinned, and nothing calls a model.

## Retrieval

[`retrieval.jsonl`](retrieval.jsonl) has 100 questions, each with the corpus
paragraphs that answer it. Each line is one JSON object:

```json
{"id": "r001", "question": "How long should an OAuth authorization code stay valid before it expires?", "gold": ["RFC6749 §4.1.2 ¶2"], "reference_answer": "...", "tags": ["oauth", "lifetime"], "difficulty": "easy"}
```

- **Coverage.** Every RFC in the corpus is the gold source for at least four
  questions. 19 questions are easy, 51 medium, and 30 hard. The hard ones are
  paraphrased far from the RFC wording, need the right one of several similar
  sections, or span two RFCs. Many depend on the exact requirement level, such
  as MUST against SHOULD.
- **How gold was chosen.** Labels come from reading paragraph text, not from
  Citebound's search, so they stay independent of the system they grade. A
  paragraph is gold if an expert would quote it to support the reference
  answer. Reference lists are never gold, because they are not searched.
- **Checked on load.** Unknown fields, duplicate ids, and gold refs that are
  missing from the corpus or sit in a reference list fail the run.

Run it:

```sh
make eval                                          # the CI gate
go run ./cmd/citebound eval retrieval -out report.json   # with per-question results
```

| Metric | Meaning |
| --- | --- |
| recall@5 | Share of each question's gold paragraphs in the top 5, averaged |
| hit rate@5 | Share of questions with at least one gold paragraph in the top 5 |
| MRR@10 | Mean of 1 / rank of the first gold paragraph, 0 if none is in the top 10 |
| section hit@5 | Like hit rate@5, but any paragraph of a gold section counts |

[`thresholds.json`](thresholds.json) holds the gate: the scores of the current
search, rounded down. A change that lowers either score fails CI. A change
that raises them should raise the thresholds in the same pull request.

### Baseline

BM25 over paragraphs, with section titles weighted 3x:

| recall@5 | hit rate@5 | MRR@10 | section hit@5 |
| --- | --- | --- | --- |
| 0.52 | 0.65 | 0.44 | 0.69 |

35 questions have no gold paragraph in the top 5, and 26 have none in the top
10. Most are lexical misses: the question and the paragraph describe the same
rule in different words, or a long paragraph that states the rule ranks below
short ones that only mention its terms. `make eval` lists them.

## Known limitations

- **Gold is sufficient, not exhaustive.** The corpus repeats itself. For
  example, RFC 9700 restates rules from RFC 6749 and RFC 7636. A search can
  return a correct paragraph that is not labeled gold and still be scored as a
  miss.
- **One annotator.** The questions and labels were written and checked by one
  author, and difficulty ratings are subjective. A re-read of 15 random items
  against their gold text found one wrong paragraph number, which was fixed.
  Afterwards the opening text of every gold paragraph was checked.
- **One gold paragraph is a figure.** r097 depends on RFC 7636 §4.1 ¶2,
  where the 43 to 128 character rule for `code_verifier` is stated.
