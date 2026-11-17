# Answer format

Every answer, whether written by a model or by the extractive answerer, is
plain text with one claim per line. Each claim ends with a citation and a
verbatim quote from the cited paragraph:

```text
Refresh tokens are only ever sent to the authorization server. [RFC6749 §1.5 ¶2: "refresh tokens are intended for use only with authorization servers and are never sent to resource servers"]
```

| Part | Rules |
| --- | --- |
| Claim | One sentence. A leading `- ` bullet is ignored. |
| Reference | `RFC<number> §<section> ¶<paragraph>`. `RFC 6749` with a space is accepted. Paragraphs are numbered from 1 within a section; see `citebound corpus stats`. |
| Quote | Copied exactly from that one paragraph: same words, same case, no ellipses. At least 20 characters and 3 words. |

A line containing only `INSUFFICIENT_EVIDENCE` means the answerer found no
support in the corpus.

## Verification

Each claim is checked by `internal/verify` and marked **verified** or
**struck**:

| Reason struck | Meaning |
| --- | --- |
| `uncited` | The line has no citation. |
| `bad-citation` | The citation is not in the form above. |
| `unknown-ref` | The cited paragraph does not exist in the corpus. |
| `quote-too-short` | The quote is too short to be evidence. |
| `quote-not-found` | The quote is not in the cited paragraph. |

## Abstention

After verification, the answer is withheld if retrieval found nothing, the
answerer wrote `INSUFFICIENT_EVIDENCE`, no claim verified, or fewer than half
of the claims verified. Withheld claims are still shown, marked as withheld.
See [decision 0006](decisions/0006-abstention-policy.md).

## What verification proves

Verification proves the quote is real. Whether it supports the claim is a
separate question, measured by the faithfulness eval. See
[decision 0005](decisions/0005-exact-quote-verification.md).
