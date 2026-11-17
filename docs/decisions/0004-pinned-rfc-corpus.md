# 0004. Answer only from a pinned corpus of 21 RFCs

- Status: Accepted
- Date: 2026-11-17

## Context

Citebound's guarantee is that every quote it shows exists in the paragraph it
cites. That guarantee is only checkable, and only reproducible in CI, if the
source text is fixed. User uploads or live fetching would make every eval
result depend on what happened to be loaded.

## Decision

- The corpus is 21 RFCs covering OAuth 2.0 and its extensions, JOSE and JWT,
  and HTTP semantics. The list is in [`corpus/README.md`](../../corpus/README.md).
- Each file is the RFC Editor's plain-text publication, committed unmodified
  and pinned by SHA-256 in `corpus/manifest.json`.
- `go test ./...` and `citebound corpus verify` fail if any file changes.
  `.gitattributes` disables line-ending conversion so checksums hold on every
  platform.
- OpenID Connect is out of scope: its specifications are published by the
  OpenID Foundation, not as RFCs, and have different licensing.

## Consequences

- Paragraph IDs such as `RFC6749 §10.4 ¶2` never change, so eval sets and
  recorded responses stay valid.
- The repository carries about 2 MB of RFC text.
- Questions outside these RFCs must be refused or abstained on; the eval sets
  test that.
- Adding an RFC means re-running every eval, which is intended.

## Alternatives considered

- **Fetch RFCs at build time.** Smaller repository, but builds would depend on
  the network and on the RFC Editor serving identical bytes.
- **Use the XML or HTML formats.** Richer structure, but RFCs before 8650 only
  have reliable plain text, and one parser for one format is simpler to test.
- **Let users upload documents.** Much broader appeal, but it turns a
  measurable guarantee into an unmeasurable one. Out of scope for v0.1.
