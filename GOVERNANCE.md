# Governance

## Model

AIBox uses a **single-maintainer** model. The maintainer (see [MAINTAINERS.md](MAINTAINERS.md))
sets direction, reviews and merges pull requests, cuts releases and enforces the
[Code of Conduct](CODE_OF_CONDUCT.md). Decisions follow the project
[constitution](.specify/memory/constitution.md): architectural choices are recorded as ADRs in
[`docs/adr/`](docs/adr/README.md) and, where possible, decided by a declared experiment.

## Decision making

- Day-to-day changes: pull request review by the maintainer.
- Architectural changes: an ADR in the same pull request (status `Proposed` until merged).
- Constitution amendments: a pull request labeled `constitution` with a semantic version bump.

## Becoming a maintainer

Contributors who have, over at least three months:

1. landed several non-trivial pull requests (features, fixes or docs),
2. reviewed other contributors' pull requests constructively, and
3. shown good judgment in line with the constitution,

may be invited by the existing maintainer(s). New maintainers are added to
[MAINTAINERS.md](MAINTAINERS.md) and [`.github/CODEOWNERS`](.github/CODEOWNERS) by pull
request. Once there are three or more maintainers, decisions that cannot reach consensus are
taken by simple majority.

## Stepping down

Maintainers may step down at any time by pull request; inactive maintainers (no activity for
12 months) may be moved to an emeritus list.
