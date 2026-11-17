# 0001. Record decisions in `docs/decisions`

- Status: Accepted
- Date: 2026-11-17

## Context

Citebound makes claims about its own accuracy. Readers need to see why it is
built the way it is: which trade-offs were made and what evidence settled
them. That reasoning gets lost in chat logs and closed pull requests.

## Decision

We keep short, numbered decision records in `docs/decisions`. A record lands
in the same pull request as the change it describes. Records are append-only:
a changed decision gets a new record that supersedes the old one.

## Consequences

- Reviewers can check a change against the decision that motivated it.
- Writing a record takes a few minutes per significant change.
- The index in `docs/decisions/README.md` must be kept current; review checks it.

## Alternatives considered

- **Wiki pages.** They drift from the code and are not reviewed with it.
- **Long design documents only.** Useful for architecture, but too coarse to
  capture individual trade-offs. We keep `docs/architecture.md` as well.
