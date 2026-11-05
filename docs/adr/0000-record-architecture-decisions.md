# ADR-0000: Record architecture decisions

- **Status**: Accepted
- **Date**: 2026-11-05
- **Deciders**: maintainer (decision taken by the agent on the maintainer's behalf)
- **Decided by**: feature 001

## Context and problem statement

AIBox's value is the evidence behind each architecture choice (constitution principle I).
Decisions must be discoverable, reviewable in pull requests, and linked to the experiments
that confirm them.

## Decision drivers

- Decisions are reviewed like code, in the same pull request as the change.
- Each decision states its criteria and, when possible, the experiment that decides it.
- Low ceremony for a single maintainer.

## Considered options

1. Markdown ADRs in `docs/adr/` using MADR 4.
2. A wiki page of decisions.
3. Decisions only in pull request descriptions.

## Decision outcome

Chosen option: "Markdown ADRs in `docs/adr/` using MADR 4", because they live in git next to
the code, are reviewed in PRs, render on GitHub and on the docs site, and the MADR structure
(drivers, options, outcome, confirmation) maps directly to "decision criteria + planned
experiment".

### Consequences

- Good: every significant choice has a stable, linkable ID (`ADR-NNNN`).
- Good: experiment-driven ADRs can start as `Proposed — pending experiment EN` and be
  updated with results.
- Bad: ADRs must be kept in sync with the report; the index in `docs/adr/README.md` is
  maintained by hand.

### Confirmation

Every feature that the product spec maps to an ADR (spec §A.10) adds or updates it in its
PR; reviewers check the PR's "Decisions for the reviewer" section against `docs/adr/`.

## Links

- MADR: <https://adr.github.io/madr/>
- Template: [template.md](template.md)
