# Feature Specification: Open-source foundation, CI and contributor experience

**Feature Branch**: `001-repo-foundation`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "Open-source foundation for the AIBox repository: governance files, templates, repo-as-code, quality gates in CI aggregated by `ci-gate`, security workflows, dependency updates, release automation and developer experience (playbook §6, feature 001)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - First-time contributor gets productive with one command (Priority: P1)

A first-time contributor reads the README, the contributing guide and the code of conduct,
sets up the development environment with one command, runs every local check, and opens a
pull request using a template that tells them what to fill in.

**Why this priority**: AIBox is a public reference project; if a contributor cannot get to
a green local run quickly, nobody else can verify or extend it.

**Independent Test**: On a fresh clone (macOS with Homebrew or Linux with apt), run
`make dev-setup && make lint test`; all checks pass without any cloud credential.

**Acceptance Scenarios**:

1. **Given** a fresh clone on Linux or macOS, **When** the contributor runs
   `make dev-setup && make lint test`, **Then** every check passes and no cloud credential
   is requested.
2. **Given** the repository root, **When** the contributor runs `make help`, **Then** every
   available target is listed with a one-line description.
3. **Given** a contributor opens a pull request, **When** the PR form appears, **Then** it is
   pre-filled with sections for summary, stack, requirements covered, decisions, verification,
   non-verified items and a checklist.

---

### User Story 2 - Maintainer relies on a single required check (Priority: P1)

The maintainer protects the default branch with one required status check, `ci-gate`, that
summarizes every required CI job and fails if any of them fails or is cancelled.

**Why this priority**: a single, stable required check keeps branch protection simple while
jobs are added by later features.

**Independent Test**: Open one PR with a lint error and one without; `ci-gate` fails on the
first and passes on the second.

**Acceptance Scenarios**:

1. **Given** a PR containing a lint error, **When** CI runs, **Then** the lint job fails and
   `ci-gate` reports failure.
2. **Given** a clean PR, **When** CI runs, **Then** all required jobs pass and `ci-gate`
   reports success.
3. **Given** a PR whose title is not a Conventional Commit, **When** CI runs, **Then** the
   PR-title job fails and `ci-gate` fails.
4. **Given** a PR containing a commit without a `Signed-off-by` trailer, **When** CI runs,
   **Then** the DCO job names the offending commit and `ci-gate` fails.

---

### User Story 3 - Security researcher reports privately (Priority: P1)

A security researcher finds a security policy with a private reporting channel (GitHub
private vulnerability reporting) and a response-time target.

**Why this priority**: a public project must offer a safe disclosure path from day one.

**Independent Test**: Read `SECURITY.md`; it names the private channel, the supported
versions, and the acknowledgement and fix targets.

**Acceptance Scenarios**:

1. **Given** a researcher on the repository page, **When** they open `SECURITY.md`, **Then**
   they find how to report privately, what to include, and the response-time targets.

---

### User Story 4 - Maintainer releases from Conventional Commits (Priority: P2)

Releases are prepared automatically from Conventional Commits (changelog and tag), and
release artifacts are signed, accompanied by an SBOM and build provenance.

**Why this priority**: release automation is needed before v0.1.0 but not for day-to-day work.

**Independent Test**: The release configuration validates (`goreleaser check`), the release
workflow triggers only on version tags, and the release-please configuration targets the
default branch.

**Acceptance Scenarios**:

1. **Given** Conventional Commits merged to the default branch, **When** release automation
   runs, **Then** a release PR with a generated changelog is opened.
2. **Given** a version tag is pushed, **When** the release workflow runs, **Then** binaries for
   darwin/linux × amd64/arm64 are built with an SBOM, a keyless signature and a provenance
   attestation.

---

### User Story 5 - Visitor sees repository health (Priority: P2)

A visitor sees the repository's health at a glance: OpenSSF Scorecard, CI and license badges.

**Why this priority**: signals trust, but no functionality depends on it.

**Independent Test**: The README renders the three badges with working links.

**Acceptance Scenarios**:

1. **Given** the README, **When** a visitor opens it, **Then** CI, OpenSSF Scorecard and
   license badges are shown and link to their sources.

### Edge Cases

- A contributor on Linux without Homebrew: `make dev-setup` installs tools without Homebrew
  (Go toolchain, `uv`, `npm` or direct downloads) or prints the exact manual command.
- A fork without any repository secret: every required CI job passes using only the default
  `GITHUB_TOKEN` with read permissions.
- Stacked pull requests whose base is another feature branch: CI runs for any base branch.
- Merge commits created by GitHub: excluded from the DCO check.
- A job skipped because an earlier job failed: `ci-gate` treats `skipped` required jobs as
  failure unless the skip is the documented path (none in this feature).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The repository MUST contain governance files: a code of conduct (Contributor
  Covenant 2.1), a contributing guide (dev setup, Spec Kit workflow, branch/PR conventions,
  DCO sign-off, Conventional Commits), a security policy, a support guide, a governance
  document (single-maintainer model and path to becoming a maintainer), a maintainers list and
  a code-owners file.
- **FR-002**: The repository MUST provide issue forms (bug report, feature request, docs) and
  route questions to GitHub Discussions; it MUST provide a pull request template with the
  sections of playbook §5.3.
- **FR-003**: Repository configuration MUST be stored as code: the default-branch ruleset, the
  label set with a sync mechanism, the ADR log with ADR-0000 and an ADR template.
- **FR-004**: Markdown linting MUST disable line-length checks (MD013) and treat duplicate
  headings as errors only among siblings (MD024 `siblings_only`).
- **FR-005**: CI MUST run on pull requests targeting any branch and on pushes to the default
  branch, and MUST include: lint (Go, GitHub Actions, shell, YAML, Markdown, EditorConfig), Go
  unit tests with the race detector, a Conventional-Commit PR-title check, a DCO sign-off
  check, dependency review, and an aggregating `ci-gate` job.
- **FR-006**: `ci-gate` MUST depend on every required job, run even when they fail, and fail
  unless every required job succeeded.
- **FR-007**: Security workflows MUST include CodeQL (Go, Python, Actions), OpenSSF Scorecard
  (weekly and on push to the default branch, results uploaded to code scanning) and a Trivy
  filesystem/config scan.
- **FR-008**: Automated dependency updates MUST cover Go modules, Python (uv/pip), GitHub
  Actions, Docker, Terraform and Helm where supported, grouped weekly, with Conventional
  Commit prefixes.
- **FR-009**: Release automation MUST produce a changelog and tags from Conventional Commits,
  build darwin/linux × amd64/arm64 binaries with an SBOM, keyless signatures and build
  provenance attestations, and run the build workflow only on version tags.
- **FR-010**: The developer experience MUST include Makefile targets `help`, `dev-setup`,
  `lint`, `fmt`, `test`, a pre-commit configuration mirroring CI linters, and configuration
  files for each linter.
- **FR-011**: Every workflow MUST declare top-level empty or read-only permissions, grant
  least privilege per job, pin actions by full commit SHA with a version comment, and set
  concurrency groups and timeouts.
- **FR-012**: Every governance and configuration file listed above MUST be linked from the
  README or the contributing guide.
- **FR-013**: CI MUST pass in forks without any repository secret.

### Key Entities

- **Required job**: a CI job whose failure blocks merge through `ci-gate`.
- **Label**: name, color and description, defined once in a label catalog file.
- **ADR**: numbered decision record with status, context, options, decision and consequences.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A clean clone passes `make lint test` locally and `ci-gate` in CI with zero
  repository secrets.
- **SC-002**: A deliberately broken PR (lint error, bad title or missing sign-off) shows
  `ci-gate` failing within one CI run.
- **SC-003**: The OpenSSF Scorecard workflow runs successfully once the branch reaches the
  default branch (score is not gated).
- **SC-004**: 100% of the files listed in FR-001…FR-003 exist and are linked from README or
  CONTRIBUTING.
- **SC-005**: 100% of third-party actions are pinned by a 40-character commit SHA.

## Assumptions

- The project starts with a single maintainer (`@hermeto`), who is also the sole code owner.
- GitHub-hosted runners (`ubuntu-latest`) are used; no self-hosted runners.
- Application code arrives in feature 002; this feature ships only a placeholder Go program so
  Go jobs have something to build, lint and test.
- Repository settings that require admin API access (ruleset activation, security toggles,
  labels, milestones) are applied by a committed script that the maintainer runs once.
- Release workflows cannot be exercised before the stack is merged to the default branch;
  their configuration is validated statically.
