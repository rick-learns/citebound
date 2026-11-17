# HTTP API

`citebound serve` listens on `127.0.0.1:8080` by default. All responses carry
a strict `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, and
`Referrer-Policy: no-referrer`.

Paths outside `/api/` serve the web UI, if the binary was built with it
(`make build`; see [decision 0009](decisions/0009-embed-the-web-ui.md)).
Otherwise they are a plain-text 404 that says how to build it.

Errors outside a stream are JSON with a matching status:

```json
{"error": {"code": "bad_request", "message": "the question is empty"}}
```

| Code | Status | Meaning |
| --- | --- | --- |
| `bad_request` | 400, 405, 415 | The request is malformed or not UTF-8, uses the wrong method (`Allow` lists the right ones), or its media type is not `application/json`. |
| `not_found` | 404 | No such endpoint, RFC, or section. |
| `rate_limited` | 429 | The client asked too often. `Retry-After` gives the wait in seconds. See [decision 0010](decisions/0010-rate-limit-and-cap-asks.md). |
| `unavailable` | 503 | Too many questions are being answered at once (`Retry-After: 5`), or, in a stream, the answer timed out. |
| `internal` | 500 | The answer failed. |
| `budget_exhausted` | 503 | Reserved for a daily cost limit. |
| `not_recorded` | (stream only) | Replay mode has no recorded answer for the question. Only questions that retrieval finds passages for get this far; others are withheld with `no-passages`. |

## `GET /api/meta`

```json
{"version": "v0.1.0", "mode": "extractive", "model": "", "rfcs": [{"number": 6749, "title": "The OAuth 2.0 Authorization Framework"}]}
```

`mode` is `extractive` (no model), `model` (a live model), or `replay`
(a model's recorded answers, with no network). `model` names the model, such
as `openrouter/nvidia/nemotron-3-ultra-550b-a55b:free`, and is empty for
`extractive`.

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

Before streaming, a valid request can be refused with `429 rate_limited` or
`503 unavailable` by the guards in
[decision 0010](decisions/0010-rate-limit-and-cap-asks.md). `citebound serve`
sets them with `-ask-rate`, `-ask-burst`, `-max-inflight`, and
`-client-ip-header`.

The response is `text/event-stream`. Each event is one `event:` line and one
`data:` line of JSON. Events arrive in this order:

| Event | Data | When |
| --- | --- | --- |
| `retrieval` | `{"passages": [{"ref", "rfc", "rfcTitle", "section", "sectionTitle", "paragraph", "text", "figure"}]}` | Once, first. May be empty. |
| `claim` | `{"id": 0, "text": "...", "ref": "RFC6749 §10.4 ¶1", "quote": "..."}` | Per claim. `ref` is `""` for an uncited claim, or the raw text of a malformed citation. |
| `verdict` | `{"id": 0, "status": "verified" \| "struck", "reason": ""}` | Right after its claim. Reasons are listed in [the answer format](answer-format.md). |
| `abstain` | `{"code": "too-many-struck", "reason": "Too few claims could be verified to trust the answer."}` | If the answer is withheld. Codes are listed in [decision 0006](decisions/0006-abstention-policy.md). |
| `done` | `{"mode", "model", "claims", "verified", "struck", "latencyMs", "inputTokens", "outputTokens", "costUsd"}` | Last, unless an error ended the stream. `inputTokens` and `outputTokens` are what the model reported for this answer: 0 in `extractive` mode, and in `replay` mode the counts recorded with the answer. `costUsd` is not computed yet and is always 0. |
| `error` | `{"code": "unavailable", "message": "..."}` | Instead of the remaining events, if answering failed after the stream started. |

When an answer is withheld, its claims and verdicts have already been
streamed. Show them as withheld, not as the answer.

```sh
curl -N -H 'Content-Type: application/json' \
  -d '{"question": "Should access tokens be sent in the URI query?"}' \
  http://127.0.0.1:8080/api/ask
```
