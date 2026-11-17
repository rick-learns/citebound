# Contributing to Citebound

Thanks for helping. This guide covers how a change goes from an idea to a
release. It applies to every change, including the maintainers' own.

## How a change lands

```text
issue ──► branch ──► small PR ──► CI + review ──► squash merge ──► release PR
```

1. **Start from an issue.** Bugs and features are tracked as issues, grouped
   into milestones. If an issue does not exist yet, open one first so the
   problem is agreed before the solution.
2. **Branch from `main`.** Name the branch after the change type and topic,
   for example `feat/quote-verifier` or `fix/sse-reconnect`.
3. **Keep PRs small.** Aim for under 400 changed lines, excluding generated
   files and fixtures. Split larger work into a sequence of PRs that each
   leave `main` working.
4. **Pass CI and review.** A PR merges only when every CI check is green and
   review has no open blocking findings.
5. **Squash merge.** The PR title becomes the single commit on `main`, so it
   must follow [Conventional Commits](#commit-messages).
6. **Release.** [Release Please](https://github.com/googleapis/release-please)
   keeps a release PR open that collects changes into `CHANGELOG.md`. Merging
   it tags a version.

## Commit messages

Commit subjects and PR titles follow
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```text
<type>(<optional scope>): <summary in the imperative, under 100 characters>
```

| Type | Use for | In changelog |
| --- | --- | --- |
| `feat` | New behavior a user can see | Yes |
| `fix` | A bug fix | Yes |
| `perf` | Faster or cheaper, same behavior | Yes |
| `eval` | Eval sets, metrics, and gates | Yes |
| `docs` | Documentation only | Yes |
| `refactor` | Code change with no behavior change | No |
| `test` | Tests only | No |
| `build` | Build system and dependencies | No |
| `ci` | CI configuration | No |
| `chore` | Anything else | No |

Add `!` after the type for a breaking change. CI checks every commit subject
on the branch and the PR title.

A good summary says why the change matters, not only what it touches:

- Good: `fix(verifier): stop accepting quotes that span two paragraphs`
- Weak: `fix(verifier): update matcher`

## Local setup

You need Go (version in `go.mod`), Node.js 24, and
[golangci-lint](https://golangci-lint.run/) v2.

```sh
make web-install   # install web dependencies once
make check         # every gate CI runs
make build         # build bin/citebound with the web UI embedded
```

## Tests and evals

- Every behavior change comes with a test that fails without it.
- Tests never call the network or a model provider. Model calls go through
  recorded responses.
- Changes that move an eval metric must say so in the PR description. A
  change that lowers a gated threshold needs a decision record.

## Decisions and docs

- A choice that is expensive to reverse gets a record in
  [`docs/decisions`](docs/decisions/README.md), in the same PR.
- Docs are part of the change. If behavior changes, the README and the
  relevant page in `docs/` change in the same PR.

## Review

Every PR gets a review focused on correctness: bugs, broken callers, and
mismatch with the issue. Style is left to the linters. Findings are
classified as `blocker`, `should`, or `nit`; blockers and shoulds are fixed
or answered with evidence before merge.

Much of this project is written with AI coding assistants. That changes
nothing about the bar: the same CI, the same tests, and an independent
review, done by a model from a different provider than the one that wrote
the change. PR descriptions record what review found and how it was handled.

## Code of conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). Report
security issues as described in [SECURITY.md](SECURITY.md).
