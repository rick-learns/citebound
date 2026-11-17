# Security policy

## Reporting a vulnerability

Please report security issues privately by email to
[hello@playcyclorama.com](mailto:hello@playcyclorama.com), not in a public
issue. Include steps to reproduce and the version or commit you tested.

You should get a reply within five working days. We will agree on a
disclosure date with you once a fix is ready.

## Scope

In scope:

- The `citebound` binary and its HTTP server
- The web UI
- The abuse and cost guards that protect a public deployment
- Prompt-injection paths that make the server produce a citation the
  verifier should have struck

Out of scope:

- Wrong answers that carry no citation, or that the verifier correctly
  strikes. These are quality issues; please open a regular issue.
- Vulnerabilities in third-party model providers.

## Supported versions

Only the latest release receives security fixes.
