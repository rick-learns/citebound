# Claim support rubric

Each label judges one verified claim from a recorded answer. The verifier has
already confirmed that the quote appears in the cited paragraph. The label is
the part the verifier cannot do: does the cited text support the claim?

Judge the claim against the cited paragraph only. Not other RFC text, not your
own knowledge, not the question. Read the quote in the context of its paragraph.

Labels:

- `supported`: the paragraph states or directly entails everything the claim
  asserts. Rewording, summarizing, and an obvious restatement are fine.
  Normative strength must match: a claim that something MUST happen is not
  supported by text that says SHOULD or MAY.
- `partial`: the paragraph supports the core of the claim, but the claim adds
  something it does not say: an extra condition, a stronger modal (SHOULD
  presented as MUST), a broader scope, a reason or consequence the paragraph
  does not give, or a detail from elsewhere.
- `unsupported`: the paragraph does not support the claim's main assertion,
  or contradicts it, or the quote is used out of context (for example, it
  describes a different mechanism, an attack rather than a requirement, or a
  case the claim ignores).

A claim that just repeats a sentence of the paragraph is `supported`, even if
it does not answer the question well. Answer quality is not being judged.

Be strict and consistent. When torn between two labels, pick the stricter one
and say why in the note. A `partial` or `unsupported` label needs a one-sentence
note saying what the paragraph does not support.
