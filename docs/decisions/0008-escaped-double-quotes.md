# 0008. Read an escaped double quote in a quote as a double quote

- Status: Accepted
- Date: 2026-11-17

## Context

RFCs often put parameter and claim names in double quotes, as in
`the "aud" claim`. When a model quotes such text, it tends to escape them:
`[RFC9068 §4 ¶8: "The resource server MUST validate that the \"aud\" claim
contains ..."]`. Nemotron 3 Ultra did this on every quote in its JWT
access-token answer, including after the prompt asked it to write plain double
quotes. The question and passages reach the model as JSON, which may encourage
the habit.

Under [decision 0005](0005-exact-quote-verification.md), `\"aud\"` is not in the
paragraph, so all three real quotes were struck as `quote-not-found` and a
correct answer was withheld. That is a false negative caused by the answer
format, not by the evidence.

The answer format itself already handles plain double quotes: the parser
captures everything between the first `"` after the citation and the last `"`
before the closing `]`.

## Decision

When the quote as written is not in the cited paragraph, the verifier tries
again with each `\"` read as `"`. A claim verifies if either form is a
substring of the normalized paragraph.

Only `\"` is read this way. Other backslash sequences are compared as written.
The quote is tried as written first because a few RFC examples contain a
literal `\"`, such as `title="Login to \"apps\""` in RFC 9110.

## Consequences

- Correct claims that quote parameter names are no longer struck for their
  escaping. The recorded JWT answer verifies.
- Soundness is unchanged: either form must appear word for word in the cited
  paragraph, so nothing that is not in the RFC can verify.
- Claim events still carry the quote as the model wrote it. A client that
  highlights the quote in the paragraph must read `\"` the same way.

## Alternatives considered

- **Unescape in the parser.** Simpler, but it would break quotes of RFC
  examples that contain a literal `\"`.
- **Add `\"` to `Normalize`.** Normalization applies to the paragraph too, so
  it would change RFC text, not only the quote.
- **Rely on the prompt.** Tried: the model escaped the quotes anyway.
- **Full JSON string unescaping** (`\\`, `\n`, `\u0022`). No model has needed
  it, and each extra rule widens what counts as "verbatim".
