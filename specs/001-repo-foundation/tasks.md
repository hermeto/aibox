# Tasks: Open-source foundation, CI and contributor experience

**Input**: Design documents from `/specs/001-repo-foundation/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: included where the plan names them (placeholder Go test, DCO script test,
actionlint, goreleaser check).

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [ ] T001 Initialize Go module `github.com/hermeto/aibox` (`go 1.27.3`) in go.mod
- [ ] T002 [P] Add placeholder `cmd/aibox/main.go` and `cmd/aibox/main_test.go`
- [ ] T003 [P] Add linter configs `.golangci.yml`, `.markdownlint-cli2.yaml` (MD013 off, MD024 siblings_only), `.yamllint`

## Phase 2: Foundational

- [ ] T004 Makefile with `help`, `tools`, `dev-setup`, `lint` (+ `lint-*`), `fmt`, `test`; pinned tool versions
- [ ] T005 [P] `hack/dev-setup.sh` (Linux/macOS, no Homebrew required)
- [ ] T006 [P] `.pre-commit-config.yaml` with local hooks calling `make lint-*`

## Phase 3: User Story 1 — contributor (P1)

- [ ] T007 [P] [US1] CODE_OF_CONDUCT.md (Contributor Covenant 2.1)
- [ ] T008 [P] [US1] CONTRIBUTING.md (dev setup, Spec Kit loop, branches/PRs, DCO, Conventional Commits)
- [ ] T009 [P] [US1] SUPPORT.md, GOVERNANCE.md, MAINTAINERS.md, .github/CODEOWNERS
- [ ] T010 [P] [US1] Issue forms `.github/ISSUE_TEMPLATE/{bug_report,feature_request,docs}.yml` + `config.yml`
- [ ] T011 [P] [US1] `.github/PULL_REQUEST_TEMPLATE.md` with playbook §5.3 sections
- [ ] T012 [US1] README links to every governance/config file

## Phase 4: User Story 2 — maintainer quality gates (P1)

- [ ] T013 [US2] Test first: `hack/ci/dco-check_test.sh` covering signed, unsigned, mismatched-email and merge commits
- [ ] T014 [US2] `hack/ci/dco-check.sh` passes T013
- [ ] T015 [US2] `hack/ci/ci-gate.sh` (results JSON → pass/fail, allowed-skip list)
- [ ] T016 [US2] `.github/workflows/ci.yml`: lint, test, pr-title, dco, dependency-review, trivy, ci-gate
- [ ] T017 [P] [US2] `.github/rulesets/main.json`, `.github/labels.yml`, `.github/workflows/labels.yml`
- [ ] T018 [P] [US2] `hack/github/setup-repo.sh` (settings, security, ruleset, labels, milestones)
- [ ] T028 [US2] `hack/ci/check-action-pins.sh` + `make lint-pins` (fails on any `uses:` not pinned to a 40-char SHA with a version comment) — covers SC-005/FR-011
- [ ] T019 [P] [US2] `docs/adr/0000-record-architecture-decisions.md`, `docs/adr/template.md`, `docs/adr/README.md`

## Phase 5: User Story 3 — security reporter (P1)

- [ ] T020 [P] [US3] SECURITY.md with private vulnerability reporting and response targets
- [ ] T021 [P] [US3] `.github/workflows/codeql.yml` (go, actions) and `.github/workflows/scorecard.yml`
- [ ] T022 [P] [US3] `.github/dependabot.yml` (gomod, uv/pip, github-actions, docker, terraform, helm; grouped weekly)

## Phase 6: User Story 4 — releases (P2)

- [ ] T023 [US4] `release-please-config.json`, `.release-please-manifest.json`, `.github/workflows/release-please.yml`
- [ ] T024 [US4] `.goreleaser.yaml` (darwin/linux × amd64/arm64, ldflags, syft SBOM, cosign keyless) and `.github/workflows/release.yml` (tags only, attestations)

## Phase 7: User Story 5 — health badges (P2)

- [ ] T025 [US5] README badges (CI, OpenSSF Scorecard, license)

## Phase 8: Polish

- [ ] T026 Run `make lint test`, `actionlint`, `pre-commit run --all-files`, `goreleaser check`
- [ ] T027 Update docs/agent/PROGRESS.md

## Analysis log

`/speckit-analyze` pass 1 (2026-11-05): 1 HIGH finding — SC-005/FR-011 (actions pinned by SHA) had no enforcing task → added T028. No CRITICAL findings; coverage 13/13 FR, 5/5 SC. Pass 2: clean.

## Dependencies

Phase 1 → Phase 2 → stories (US1–US5 independent) → Polish. T013 before T014.
