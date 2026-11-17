# Limitations

Where Citebound's guarantee stops, with a real or hand-written example of
each. Each example is pinned by a CLI test (`TestRun` in
`cmd/citebound/main_test.go`), so if the behavior changes, the test fails and
this page has to change with it.

The guarantee is narrow: every quote shown as verified appears, after
[normalization](decisions/0005-exact-quote-verification.md), in the paragraph
it cites. Everything below is outside it.

## A real quote can sit beside a false claim

[`examples/false-claim.txt`](../examples/false-claim.txt):

```console
$ citebound check examples/false-claim.txt
✓ Refresh tokens may be sent to resource servers.
  RFC6749 §1.5 ¶2

1 of 1 claims verified.
```

The quote is real, and it says the opposite: refresh tokens "are never sent
to resource servers". Verification checks that the quote exists, not that it
supports the claim. This is the most important limit. A model that quotes
correctly but reasons badly gets every check mark.

**Measured by:** claim support in the eval scorecard (#11), with labeled
claims, kept apart from quotation validity.

## A real quote can be struck because the model escaped it

[`examples/escaped-figure.txt`](../examples/escaped-figure.txt) is Nemotron 3
Ultra's answer to "How can I bind the authorization code itself to my DPoP
key?" (eval question r071). It quotes the request figure from RFC 9449 §10 ¶4
correctly, but writes it as a JSON string: each line-continuation backslash
and line break becomes the literal text `\\\n`.

```console
$ citebound check examples/escaped-figure.txt
✗ The client includes the dpop_jkt parameter in the authorization request with the JWK Thumbprint of its DPoP public key.
  struck: quote-not-found
```

This is a false strike, and the claim is true. The verifier undoes only one
escape, `\"` ([decision 0008](decisions/0008-escaped-double-quotes.md)),
because undoing more, such as `\\` or `\n`, would also change RFC text that
really contains backslashes. The other four claims in that answer verified,
so the answer was shown with this one struck.

## A reply cut off at the token limit ends in a struck line

[`examples/cut-off.txt`](../examples/cut-off.txt) is Nemotron's answer to
whether to fetch a JWT's `jku` URL (r049). The model used its whole
4,096-token budget, and the reply stops mid-sentence:

```console
$ citebound check examples/cut-off.txt
✓ Blindly following a "jku" or "x5u" header URL can lead to server-side request forgery (SSRF) attacks.
  RFC8725 §3.10 ¶2

✗ Applications should protect against such attacks, for example by matching the URL to
  struck: uncited
```

The struck line is shown, so the cut is visible, and the verified claim is
still correct. But nothing tells the user that the answer is incomplete.
Hidden reasoning tokens count against the same budget. Two of 180 recorded
replies were cut off this way.

## A paraphrase is struck, even when it is right

[`examples/paraphrase.txt`](../examples/paraphrase.txt) makes a true claim,
but its quote says "soon" and "reduce" where RFC 6749 says "shortly" and
"mitigate":

```console
$ citebound check examples/paraphrase.txt
✗ Authorization codes must expire soon after they are issued.
  struck: quote-not-found
```

The verifier wants the RFC's own words, apart from whitespace and typographic
punctuation. That is the intended trade: code cannot check a paraphrase, so a
paraphrase is never shown as evidence.

## Retrieval bounds what can be answered

The answerer sees only the top passages from BM25. If the right paragraph is
not among them, the best outcome is an abstention. Retrieval finds at least
one labeled paragraph in the top five for 65% of the
[labeled questions](../evals/README.md), so many abstentions on answerable
questions start here.
