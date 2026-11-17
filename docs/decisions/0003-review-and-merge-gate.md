# 0003. Small PRs, independent review, and a squash-merge gate

- Status: Accepted
- Date: 2026-11-17

## Context

The history of this repository is part of what it demonstrates. A single
large commit hides how the work was done and makes regressions hard to
bisect. Most of the code is written with AI coding assistants, which makes
an independent check on each change more important, not less.

## Decision

- Every change starts from an issue and lands through a pull request. Nothing
  is pushed to `main` directly.
- PRs are small: one issue each, under about 400 changed lines excluding
  generated files.
- Commit subjects and PR titles follow Conventional Commits, enforced in CI.
- Each PR is reviewed for correctness by a model from a different provider
  than the one that wrote it. Findings are classified as `blocker`, `should`,
  or `nit`. Blockers and shoulds are fixed or answered with evidence; the PR
  description records the outcome.
- A PR merges only when CI is green and review has approved the current
  diff. Merges are squash merges; the PR title becomes the commit on `main`.
- Release Please turns the Conventional Commits on `main` into release PRs,
  a changelog, and version tags.

## Consequences

- `main` has one meaningful commit per change, and each links back to a PR
  with its discussion, review, and verification.
- While the repository is private on a free plan, GitHub branch protection is
  unavailable, so the merge gate is enforced by process. Turning on branch
  protection (required checks, no direct pushes) is part of making the
  repository public.
- Release PRs opened by the default `GITHUB_TOKEN` do not trigger CI. Every
  commit they collect has already passed CI on its own PR, and the release PR
  only changes the changelog and manifest.

## Alternatives considered

- **Rebase merges that keep every branch commit.** More history on `main`,
  but review fix-ups land there too, and PR titles stop being the unit of
  change that the changelog is built from.
- **Merge commits.** Non-linear history that is harder to bisect and read.
- **Self-review only.** Cheaper, but the author's blind spots are exactly
  what review is meant to catch.
