# HTTP API

`citebound serve` listens on `127.0.0.1:8080` by default. All responses carry
a strict `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, and
`Referrer-Policy: no-referrer`.

Errors are JSON with a matching status:

```json
{"error": {"code": "not_found", "message": "no such section in RFC 6749"}}
```

| Code | Status | Meaning |
| --- | --- | --- |
| `bad_request` | 405 | The request uses the wrong method; `Allow` lists the right ones. |
| `not_found` | 404 | No such endpoint, RFC, or section. |

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
