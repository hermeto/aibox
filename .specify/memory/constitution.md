# AIBox — AI Platform in a Box Constitution

## Core Principles

### I. Evidence over opinion

Every architectural choice MUST either be measured by a declared experiment or be recorded as
an Architecture Decision Record (ADR) in `docs/adr/` with explicit decision criteria and a
planned experiment that will confirm or overturn it. Claims in documentation (performance,
cost, latency, "better than") MUST link to data: a benchmark result, a report section or an
upstream source.

Rationale: AIBox's product is the evidence that each component was chosen because of a
number; an unmeasured claim undermines the whole report.

### II. Reproducible by default

All dependencies MUST be pinned: Go and Python lock files, Terraform provider lock files,
Helm chart versions, container image digests, GitHub Action commit SHAs and model revisions
(commit SHAs). Floating references such as `latest` tags or unpinned branches MUST NOT be
used. Benchmark results MUST carry complete metadata (commit, image digests, versions,
hardware, flags, dataset hashes) or they are rejected by tooling.

Rationale: another person must be able to reproduce an experiment within 10% on TTFT p50.

### III. Local-first

Every feature MUST work in the `local` profile on kind without GPUs or cloud credentials
and MUST be covered by CI on every pull request. Building, linting and testing MUST NOT
require cloud credentials or repository secrets.

Rationale: zero-cost development, demos and CI keep the project contributable and verifiable.

### IV. One command, zero orphans

Cloud environments MUST be created and destroyed by one command, protected by a mandatory
budget and time-to-live (TTL), and verified orphan-free by a tag-based checker after
teardown. Automation MUST NOT spend cloud money without an explicit, approved human action
(no unattended `apply`, no auto-triggered cloud workflows).

Rationale: GPU clusters cost tens of dollars per hour; nothing may be left billing.

### V. Declarative and GitOps

Platform state MUST live in git under `platform/` and be delivered by Argo CD
(app-of-apps). Imperative steps are limited to bootstrap (cluster creation, Argo CD install,
root Application) and MUST be idempotent.

Rationale: what runs is what is reviewed; drift is visible and reversible.

### VI. Test what matters

The project MUST have: unit tests for CLI logic with at least 80% coverage on core
packages; contract tests for external interfaces (runtime metrics, OpenAI-compatible API);
end-to-end tests on the `local` profile; golden tests for generated artifacts (help output,
rendered manifests, reports). Tests MUST be written before or with the code, never after the
pull request is opened.

Rationale: a reference platform is only as credible as its verification.

### VII. Observability is a feature

Every component MUST expose metrics; every inference request MUST be traceable from the
gateway to the runtime where the pinned versions allow it (gaps documented). Dashboards and
alerts MUST be code, stored in git and reviewed like code.

Rationale: every number in the report must also be visible live.

### VIII. Secure by default

No secrets in git. Identities MUST be least-privilege (EKS Pod Identity, scoped IAM, read-only
CI tokens). Workloads SHOULD run with restricted Pod Security where the component allows it.
Releases MUST be signed, with an SBOM and build provenance. CI MUST run supply-chain checks
(CodeQL, OpenSSF Scorecard, Trivy, dependency review).

Rationale: a public reference project is copied; its defaults must be safe to copy.

### IX. Simplicity and small changes

Prefer the simplest option that can be measured. No speculative abstractions. Pull requests
MUST stay small and focused on one feature or one iteration (target < 800 changed lines
excluding generated files, lock files and Spec Kit artifacts). v2 ideas go to the roadmap,
not the code.

Rationale: small, focused changes are reviewable by a single maintainer.

### X. Open-source citizenship

Commits MUST follow Conventional Commits and carry a DCO sign-off (`Signed-off-by`).
Documentation MUST be updated in the same pull request as the behavior it describes.
Language MUST be inclusive, and contribution paths welcoming and documented.

Rationale: the repository is judged by how easy it is to contribute to.

## Technology and Delivery Constraints

- Languages: Go (latest stable) for the `aibox` CLI and small tools/controllers; Python
  (latest stable, managed with uv) only for the report generator and analysis;
  Terraform-compatible HCL (OpenTofu-tested) for AWS.
- Platform components: Kustomize bases and overlays (`local`, `aws`) with pinned Helm
  charts, delivered by Argo CD app-of-apps.
- Profiles: `local` (kind, no GPU, inference simulator), `aws-dev`, `aws-bench`.
- Repository layout follows `docs/product/spec.md` §A.8.
- Cost guardrails (budget, TTL, GPU limits, tags `project=aibox`, `run-id`, `owner`,
  orphan checker) are mandatory for every cloud profile.

## Development Workflow and Quality Gates

- Every feature follows the Spec Kit loop: specify → clarify → plan → checklist → tasks →
  analyze → implement → converge, with artifacts under `specs/NNN-short-name/`.
- Every pull request passes the aggregated `ci-gate` status check (lint, tests, PR title,
  DCO, dependency review, and the end-to-end `local` profile job once it exists).
- Decisions taken on the maintainer's behalf are recorded in the spec's `## Clarifications`,
  in an ADR when architectural, and in the PR description under "Decisions for the reviewer".
- Linters, tests and CI checks MUST NOT be disabled to get green; fix the cause or open an
  issue labeled `needs-human`.

## Governance

This constitution supersedes other practices in the repository. Amendments happen via a pull
request labeled `constitution`, with a semantic version bump (MAJOR for removed or redefined
principles, MINOR for added principles or materially expanded guidance, PATCH for wording),
and a sync of dependent templates. Every plan's Constitution Check MUST pass or justify each
deviation in the plan's Complexity Tracking table. Reviewers verify compliance on every pull
request; runtime agent guidance lives in `CLAUDE.md`.

**Version**: 1.0.0 | **Ratified**: 2026-11-05 | **Last Amended**: 2026-11-05
