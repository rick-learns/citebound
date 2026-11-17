# Citebound

An RFC assistant that checks every quotation against pinned RFC text.

Ask a question about OAuth 2.0, JOSE and JWT, or HTTP. Every claim in the
answer cites one paragraph of a pinned RFC and quotes it word for word. Code,
not a model, checks that each quote really is in the paragraph it cites.
Claims that fail are struck out, and if nothing survives, Citebound says the
corpus does not support an answer.

The check proves a quote is real, not that it supports the claim beside it: a
true quote can sit next to a false claim and still pass. See
[what verification proves](docs/answer-format.md#what-verification-proves).

```console
$ citebound ask "Can a refresh token be bound to a client?"
✓ As a result, refresh tokens are indirectly certificate-bound by way of the client ID and the associated requirement for (certificate-based) authentication to the AS when issued to clients utilizing the "tls_client_auth" or "self_signed_tls_client_auth" methods of client authentication.
  RFC8705 §7.1 ¶1

✓ The refresh token is also a secret bound to the client identifier and client instance that originally requested the authorization; the refresh token also represents the original resource owner grant.
  RFC6819 §3.3 ¶3
...
```

> [!NOTE]
> Work in progress. The answer above comes from the extractive answerer,
> which quotes sentences and needs no API key. Model answerers, the web UI,
> and the eval scorecard are next. See the
> [milestones](https://github.com/rick-learns/citebound/milestones).

## How it works

1. **Corpus.** 21 RFCs, pinned as plain text with SHA-256 checksums, are split
   into sections and paragraphs with stable IDs such as `RFC6749 §10.4 ¶2`.
   See [`corpus/`](corpus/README.md).
2. **Retrieval.** An in-memory SQLite FTS5 index ranks paragraphs with BM25.
3. **Answer.** An answerer writes one claim per line, each ending in a
   citation and a verbatim quote. See [the answer format](docs/answer-format.md).
4. **Verification.** Each quote must appear in the cited paragraph, exactly,
   apart from whitespace and typographic punctuation. Anything else is
   struck, with a reason. See
   [decision 0005](docs/decisions/0005-exact-quote-verification.md).
5. **Abstention.** If no claim verifies, or fewer than half do, the answer is
   withheld. See [decision 0006](docs/decisions/0006-abstention-policy.md).

Verification proves that a quote is real. It does not prove that the quote
supports the claim; that is what the evals measure.

## Try it

Requires Go 1.26 or later.

```sh
go run ./cmd/citebound ask "Should access tokens be sent in the URI query?"
go run ./cmd/citebound search "DPoP proof nonce"
go run ./cmd/citebound corpus stats
go run ./cmd/citebound serve   # HTTP API on 127.0.0.1:8080
```

## Documentation

- [Answer format](docs/answer-format.md)
- [HTTP API](docs/api.md)
- [Decision records](docs/decisions/README.md)
- [Corpus and parsing](corpus/README.md)
- [Contributing](CONTRIBUTING.md) and [security policy](SECURITY.md)

## License

Code: see [LICENSE](LICENSE). RFC text: see [`corpus/README.md`](corpus/README.md).
