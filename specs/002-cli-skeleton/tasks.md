# Tasks: `aibox` CLI, profiles and preflight checks

**Input**: Design documents from `/specs/002-cli-skeleton/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: required by the constitution (≥ 80% on profile and doctor; golden files).

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 Add dependencies (cobra, sigs.k8s.io/yaml, jsonschema/v6, aws-sdk-go-v2 modules) to go.mod
- [X] T002 [P] `internal/version/version.go` with ldflags-injected Version/Commit/Date; update `.goreleaser.yaml` ldflags

## Phase 2: Foundational

- [X] T003 [P] `internal/exec` Runner interface, Real, DryRun, Fake (+ tests)
- [X] T004 [P] `internal/output` text/JSON printer (+ tests)
- [X] T005 `internal/cli/errors.go` typed errors, `ExitCode` mapping (0/1/2/64) and JSON error object on stdout in `--output json` mode (+ tests)

## Phase 3: User Story 1 — command tree (P1)

- [X] T006 [US1] Golden test for `aibox --help` and not-implemented behavior in `internal/cli/root_test.go`
- [X] T007 [US1] `internal/cli/root.go` root command, global flags, logger setup
- [X] T008 [US1] `internal/cli/notimpl.go` stubs for up, down, models, smoke, bench, report, cost (exit 2)
- [X] T009 [US1] `internal/cli/version.go` (`aibox version`, text/JSON)
- [X] T010 [US1] `cmd/aibox/main.go` wiring and exit codes

## Phase 4: User Story 3 — profiles (P1)

- [X] T011 [US3] `schemas/profile.schema.json` + `schemas/embed.go`
- [X] T012 [P] [US3] `profiles/local.yaml`, `profiles/aws-dev.yaml`, `profiles/aws-bench.yaml`
- [X] T013 [US3] Tests first: `internal/profile/load_test.go` with valid profiles and invalid fixtures in `internal/profile/testdata/`
- [X] T014 [US3] `internal/profile` types, resolution (name/path, repo root, `AIBOX_PROFILES_DIR`) and validation with JSON paths

## Phase 5: User Story 2 — local doctor (P1)

- [X] T015 [US2] Tests first: `internal/doctor/local_test.go` (all pass, daemon down, tool too old, low memory) + golden JSON
- [X] T016 [US2] `internal/doctor/doctor.go` report model; `tools.go` version parsing; `local.go` checks via Runner
- [X] T017 [US2] `internal/cli/doctor.go` command (text table / JSON, exit 1 on fail)

## Phase 6: User Story 4 — AWS doctor (P2)

- [X] T018 [US4] Tests first: `internal/doctor/aws_test.go` with a fake AWS client (no creds, quota low, type not offered, all good)
- [X] T019 [US4] `internal/doctor/aws.go` checks + `internal/doctor/awsclient.go` SDK-backed client (read-only)

## Phase 7: User Story 5 — Makefile wrappers (P2)

- [X] T020 [US5] Makefile `build` and wrappers `up down models-sync smoke bench report cost` with `PROFILE`

## Phase 8: Polish

- [X] T021 [P] `docs/adr/0008-cli-in-go-with-cobra.md`; ADR index
- [X] T022 [P] CI job `cli` (doctor --profile local, `up` exits 2) added to `ci-gate`
- [X] T023 [P] CONTRIBUTING/README: CLI usage section
- [X] T024 Verify `go test ./... -race -cover` (≥ 80% profile/doctor), `make lint`, cross-compile `GOOS=linux GOARCH=arm64` and `GOOS=darwin GOARCH=arm64`, definition of done
- [X] T025 Update docs/agent/PROGRESS.md

## Analysis log

`/speckit-analyze` pass 1 (2026-11-05): 0 CRITICAL, 1 HIGH, 1 MEDIUM. HIGH — edge case "JSON error object in `--output json` mode" had no task → folded into T005. MEDIUM — edge case "Linux arm64" had no verification → cross-compilation added to T024. Coverage: FR 11/11, SC 5/5. Pass 2: clean.

## Dependencies

Setup → Foundational → US1 → US3 → US2 → US4 → US5 → Polish. Tests precede implementation
within each story.

## Convergence log

`/speckit-converge` pass 1 (2026-11-05): 0 findings. Evidence: FR-001/002 `internal/cli/root.go`,
`notimpl.go` (golden `help.golden`, `TestNotImplementedExit2`); FR-003/004 `schemas/profile.schema.json`,
`internal/profile` (`TestInvalidProfiles`); FR-005/006 `internal/doctor/local.go`, `tools.go`;
FR-007 `internal/doctor/aws.go`, `internal/awsapi` (read-only calls only); FR-008 `internal/exec`;
FR-009 `internal/version` + `.goreleaser.yaml`; FR-010 Makefile wrappers; FR-011 slog on stderr.
SC-001 coverage: profile 85.0%, doctor 98.3%. SC-002 enforced by the CI `cli` job.
