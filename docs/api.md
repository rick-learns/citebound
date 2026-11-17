# HTTP API

`citebound serve` listens on `127.0.0.1:8080` by default. All responses carry
a strict `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, and
`Referrer-Policy: no-referrer`.

Errors outside a stream are JSON with a matching status:

```json
{"error": {"code": "bad_request", "message": "the question is empty"}}
```

| Code | Status | Meaning |
| --- | --- | --- |
| `bad_request` | 400, 405, 415 | The request is malformed, uses the wrong method (`Allow` lists the right ones), or is not JSON. |
| `not_found` | 404 | No such endpoint, RFC, or section. |
| `rate_limited`, `budget_exhausted` | 429, 503 | Reserved for the public demo's guards. |
| `unavailable`, `internal` | 503, 500 | The answer timed out or failed. |

## `GET /api/meta`

```json
{"version": "v0.1.0", "mode": "extractive", "model": "", "rfcs": [{"number": 6749, "title": "The OAuth 2.0 Authorization Framework"}]}
```

## `GET /api/section?rfc=6749&section=10.4`

Every paragraph of one section, for the citation panel. Figures keep their
line breaks; prose is one line.

```json
{"rfc": 6749, "rfcTitle": "The OAuth 2.0 Authorization Framework", "section": "10.4", "sectionTitle": "Refresh Tokens",
 "paragraphs": [{"ref": "RFC6749 §10.4 ¶1", "paragraph": 1, "text": "Authorization servers MAY issue refresh tokens ...", "figure": false}]}
```

## `POST /api/ask`

Request: `Content-Type: application/json`, body `{"question": "..."}`, at
most 1,000 characters. Unknown fields are rejected.

The response is `text/event-stream`. Each event is one `event:` line and one
`data:` line of JSON. Events arrive in this order:

| Event | Data | When |
| --- | --- | --- |
| `retrieval` | `{"passages": [{"ref", "rfc", "rfcTitle", "section", "sectionTitle", "paragraph", "text", "figure"}]}` | Once, first. May be empty. |
| `claim` | `{"id": 0, "text": "...", "ref": "RFC6749 §10.4 ¶1", "quote": "..."}` | Per claim. `ref` is `""` for an uncited claim, or the raw text of a malformed citation. |
| `verdict` | `{"id": 0, "status": "verified" \| "struck", "reason": ""}` | Right after its claim. Reasons are listed in [the answer format](answer-format.md). |
| `abstain` | `{"code": "too-many-struck", "reason": "Too few claims could be verified to trust the answer."}` | If the answer is withheld. Codes are listed in [decision 0006](decisions/0006-abstention-policy.md). |
| `done` | `{"mode", "model", "claims", "verified", "struck", "latencyMs", "inputTokens", "outputTokens", "costUsd"}` | Last, unless an error ended the stream. |
| `error` | `{"code": "unavailable", "message": "..."}` | Instead of the remaining events, if answering failed after the stream started. |

When an answer is withheld, its claims and verdicts have already been
streamed. Show them as withheld, not as the answer.

```sh
curl -N -H 'Content-Type: application/json' \
  -d '{"question": "Should access tokens be sent in the URI query?"}' \
  http://127.0.0.1:8080/api/ask
```
