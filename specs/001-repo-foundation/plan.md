# Implementation Plan: Open-source foundation, CI and contributor experience

**Branch**: `001-repo-foundation` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/001-repo-foundation/spec.md`

## Summary

Ship the repository's open-source foundation: governance files, issue/PR templates,
repo-as-code (ruleset, labels, ADR log), and GitHub Actions workflows for quality gates
(`ci.yml` with lint, test, pr-title, dco, dependency-review and the aggregating `ci-gate`),
security (`codeql.yml`, `scorecard.yml`, Trivy inside `ci.yml`), dependency updates
(Dependabot), release automation (`release-please.yml`, `release.yml` with GoReleaser, syft,
cosign and artifact attestations) and label sync (`labels.yml`). A Makefile is the single
entry point for `help`, `dev-setup`, `tools`, `lint`, `fmt` and `test`, shared by local runs,
pre-commit and CI. A placeholder Go module (`github.com/hermeto/aibox`, `cmd/aibox`) gives
the Go jobs something to build, lint and test until feature 002 replaces it.

## Technical Context

**Language/Version**: Go 1.27.3 (latest stable, `go.mod`), Bash for small CI/dev scripts,
YAML for workflows and config.

**Primary Dependencies**: GitHub Actions (pinned by SHA, see research.md), golangci-lint
v2.15.0, actionlint v1.7.12, editorconfig-checker v3.11.3, shellcheck (shellcheck-py
0.11.0.1), yamllint 1.38.0, markdownlint-cli2 0.24.0, pre-commit 4.7.0, Trivy 0.76.1 (image by
digest), GoReleaser v2.20.0, syft v1.56.0, cosign v3.1.4, release-please v17 (action v5).

**Storage**: N/A.

**Testing**: `go test -race ./...` (placeholder test), `actionlint` for workflows,
`goreleaser check`, a shell test for the DCO script (`hack/ci/dco-check_test.sh`) run by
`make test`.

**Target Platform**: GitHub-hosted `ubuntu-latest` runners; contributors on macOS and Linux.

**Project Type**: Repository foundation (CI/CD + docs); CLI placeholder.

**Performance Goals**: `ci.yml` required jobs complete in ≤ 10 minutes on a cold cache.

**Constraints**: No repository secrets (fork-safe); top-level `permissions: {}`; actions
pinned by full SHA with version comment; every job has `timeout-minutes`; workflows use
`concurrency` groups.

**Scale/Scope**: ~45 files, single maintainer.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
| --- | --- | --- |
| I. Evidence over opinion | Pass | ADR-0000 establishes the ADR log and MADR template. |
| II. Reproducible by default | Pass | Actions pinned by SHA, tools pinned in Makefile, Trivy by digest, Go version pinned in `go.mod`. |
| III. Local-first | Pass | `make lint test` runs locally with no cloud credential; CI needs no secret. |
| IV. One command, zero orphans | N/A | No cloud resources in this feature. |
| V. Declarative and GitOps | N/A | No platform state yet. |
| VI. Test what matters | Pass | Go placeholder test, DCO script test, `goreleaser check`, actionlint. |
| VII. Observability is a feature | N/A | No runtime components. |
| VIII. Secure by default | Pass | Least-privilege workflows, CodeQL, Scorecard, Trivy, dependency review, signed releases with SBOM/provenance. |
| IX. Simplicity and small changes | Pass | Shell/Makefile glue only; no custom actions. |
| X. Open-source citizenship | Pass | Conventional Commit PR titles, DCO check, CoC, CONTRIBUTING. |

Post-design re-check: unchanged (all Pass/N/A).

## Project Structure

### Documentation (this feature)

```text
specs/001-repo-foundation/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── ci-gate.md
│   └── make-targets.md
├── checklists/
│   ├── requirements.md
│   └── quality.md
└── tasks.md
```

### Source Code (repository root)

```text
.github/
├── CODEOWNERS
├── PULL_REQUEST_TEMPLATE.md
├── ISSUE_TEMPLATE/{bug_report.yml,feature_request.yml,docs.yml,config.yml}
├── dependabot.yml
├── labels.yml
├── rulesets/main.json
└── workflows/{ci.yml,codeql.yml,scorecard.yml,release-please.yml,release.yml,labels.yml}
cmd/aibox/main.go            # placeholder, replaced in 002
cmd/aibox/main_test.go
docs/adr/{0000-record-architecture-decisions.md,template.md,README.md}
hack/ci/{dco-check.sh,dco-check_test.sh,ci-gate.sh}
hack/dev-setup.sh
hack/github/setup-repo.sh    # maintainer runs once (admin settings, ruleset, labels, milestones)
.golangci.yml .markdownlint-cli2.yaml .yamllint .pre-commit-config.yaml
.goreleaser.yaml release-please-config.json .release-please-manifest.json
CODE_OF_CONDUCT.md CONTRIBUTING.md SECURITY.md SUPPORT.md GOVERNANCE.md MAINTAINERS.md
Makefile go.mod
```

**Structure Decision**: Single repository root per Appendix A §8; CI glue lives in
`hack/ci/` as small, tested shell scripts so workflows stay declarative.

## Complexity Tracking

No constitution violations to justify.
