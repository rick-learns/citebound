# 0005. Verify citations by exact quote match, per paragraph

- Status: Accepted
- Date: 2026-11-17

## Context

A citation is only useful if a reader can trust that the cited text says
what the answer quotes. Models paraphrase, merge sentences from different
places, and invent section numbers. We need a check that is cheap,
deterministic, and impossible to argue with.

## Decision

Every claim in an answer carries a reference (`RFC6749 §10.4 ¶2`) and a
verbatim quote. A claim is **verified** only if the quote, after
normalization, is a substring of the cited paragraph. Otherwise it is
**struck**, with one of these reasons: `uncited`, `bad-citation`,
`unknown-ref`, `quote-too-short`, or `quote-not-found`.

Normalization is deliberately narrow:

- Typographic quotes and dashes become ASCII; runs of whitespace collapse to
  one space; trailing `.,;:` is ignored.
- **Case is kept.** RFC 2119 keywords make "MUST" and "must" different
  claims.
- **No ellipses.** A quote is one contiguous span. "A ... B" fails.
- **One paragraph.** A quote that runs across two paragraphs fails, so the
  reference always points to all of the evidence.
- **Minimum size.** At least 20 characters and 3 words, so a quote like
  "MUST" or "the client" cannot verify anything.

## Consequences

- The check is deterministic and fast, and needs no model. It runs on every
  claim of every answer, and in CI on every eval case.
- It proves that the quote exists where the answer says it does. It does
  **not** prove that the quote supports the claim. A claim can misrepresent
  a real quote. That gap is measured separately by the faithfulness eval,
  and the UI always shows the quote next to the claim so a reader can judge.
- Models must copy text exactly. Prompts and the extractive answerer are
  built around that.

## Alternatives considered

- **Fuzzy matching (edit distance, token overlap).** More forgiving of model
  slips, but any threshold admits paraphrases that change meaning, and the
  guarantee becomes "close to something in the RFC".
- **Embedding similarity.** Not deterministic across model versions, and
  similarity is not quotation.
- **A model judge for every claim.** Expensive, slow, and itself fallible.
  We use a judge for the faithfulness eval, not for the guarantee.
- **Section-level instead of paragraph-level references.** Easier for
  models, but sections in RFC 9110 run to dozens of paragraphs, which makes
  a citation much less useful to a reader.
