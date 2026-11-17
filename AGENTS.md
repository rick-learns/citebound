# Agent guide

Instructions for AI coding agents working in this repository. Humans should
read [CONTRIBUTING.md](CONTRIBUTING.md); everything there applies to agents
too.

## What this project is

Citebound answers questions about a fixed set of IETF RFCs. Every claim in an
answer carries a citation and a verbatim quote, and deterministic code checks
that the quote exists in the cited paragraph. Claims that fail are struck.
The project's value is that this guarantee is real and measured, so protect
it.

## Commands

```sh
make check   # all gates; run before every commit you push
make build   # bin/citebound
```

## Invariants

- **The verifier is deterministic.** No model calls, randomness, or network
  access in `internal/verify`. Its result for a given answer and corpus must
  never change between runs.
- **Tests are offline.** No test may reach the network or a model provider.
  Use recorded responses.
- **Gates only move with a decision.** Do not lower an eval threshold, skip a
  check, or mark a failing test as skipped without a decision record in
  `docs/decisions`.
- **No secrets in the repo.** API keys come from the environment. Never log
  them or write them into fixtures or recorded responses.
- **Corpus files are pinned.** Files under `corpus/` must match the checksums
  in the manifest. Do not edit RFC text.

## Conventions

- Conventional Commits for every commit and PR title. See CONTRIBUTING.md.
- One logical change per commit; one issue per PR; under 400 changed lines.
- Match the surrounding code. Go follows `gofmt` and the linters in
  `.golangci.yml`; TypeScript follows ESLint and Prettier in `web/`.
- Update docs and the decision index in the same PR as the change.
- Do not add personal names, emails, or account details to code, docs,
  commit messages, or fixtures. Use the project contact in SECURITY.md where
  a contact is needed.
