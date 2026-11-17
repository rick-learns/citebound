# Corpus

Citebound answers questions from this fixed set of RFCs and nothing else.
Each file is the plain-text RFC exactly as published by the
[RFC Editor](https://www.rfc-editor.org/), pinned by SHA-256 in
[`manifest.json`](manifest.json). `go test ./...` and
`citebound corpus verify` fail if any file changes.

| RFC | Title |
| --- | --- |
| [6749](https://www.rfc-editor.org/rfc/rfc6749) | The OAuth 2.0 Authorization Framework |
| [6750](https://www.rfc-editor.org/rfc/rfc6750) | The OAuth 2.0 Authorization Framework: Bearer Token Usage |
| [6819](https://www.rfc-editor.org/rfc/rfc6819) | OAuth 2.0 Threat Model and Security Considerations |
| [7009](https://www.rfc-editor.org/rfc/rfc7009) | OAuth 2.0 Token Revocation |
| [7515](https://www.rfc-editor.org/rfc/rfc7515) | JSON Web Signature (JWS) |
| [7517](https://www.rfc-editor.org/rfc/rfc7517) | JSON Web Key (JWK) |
| [7519](https://www.rfc-editor.org/rfc/rfc7519) | JSON Web Token (JWT) |
| [7636](https://www.rfc-editor.org/rfc/rfc7636) | Proof Key for Code Exchange by OAuth Public Clients |
| [7662](https://www.rfc-editor.org/rfc/rfc7662) | OAuth 2.0 Token Introspection |
| [8252](https://www.rfc-editor.org/rfc/rfc8252) | OAuth 2.0 for Native Apps |
| [8414](https://www.rfc-editor.org/rfc/rfc8414) | OAuth 2.0 Authorization Server Metadata |
| [8628](https://www.rfc-editor.org/rfc/rfc8628) | OAuth 2.0 Device Authorization Grant |
| [8693](https://www.rfc-editor.org/rfc/rfc8693) | OAuth 2.0 Token Exchange |
| [8705](https://www.rfc-editor.org/rfc/rfc8705) | OAuth 2.0 Mutual-TLS Client Authentication and Certificate-Bound Access Tokens |
| [8725](https://www.rfc-editor.org/rfc/rfc8725) | JSON Web Token Best Current Practices |
| [9068](https://www.rfc-editor.org/rfc/rfc9068) | JSON Web Token (JWT) Profile for OAuth 2.0 Access Tokens |
| [9101](https://www.rfc-editor.org/rfc/rfc9101) | The OAuth 2.0 Authorization Framework: JWT-Secured Authorization Request (JAR) |
| [9110](https://www.rfc-editor.org/rfc/rfc9110) | HTTP Semantics |
| [9126](https://www.rfc-editor.org/rfc/rfc9126) | OAuth 2.0 Pushed Authorization Requests |
| [9449](https://www.rfc-editor.org/rfc/rfc9449) | OAuth 2.0 Demonstrating Proof of Possession (DPoP) |
| [9700](https://www.rfc-editor.org/rfc/rfc9700) | Best Current Practice for OAuth 2.0 Security |

## Licensing

RFCs are published by the IETF Trust. The
[IETF Trust Legal Provisions](https://trustee.ietf.org/assets/trust-legal-provisions/)
permit copying and distributing RFCs in full, without modification. These
files are unmodified copies. They are not covered by this repository's MIT
license.

## Changing the corpus

Adding or updating an RFC is a decision: open an issue, record why in
`docs/decisions`, and update the file, `manifest.json`, and this table in the
same pull request.
