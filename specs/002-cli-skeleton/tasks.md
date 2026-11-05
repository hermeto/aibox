# Tasks: `aibox` CLI, profiles and preflight checks

**Input**: Design documents from `/specs/002-cli-skeleton/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: required by the constitution (≥ 80% on profile and doctor; golden files).

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [ ] T001 Add dependencies (cobra, sigs.k8s.io/yaml, jsonschema/v6, aws-sdk-go-v2 modules) to go.mod
- [ ] T002 [P] `internal/version/version.go` with ldflags-injected Version/Commit/Date; update `.goreleaser.yaml` ldflags

## Phase 2: Foundational

- [ ] T003 [P] `internal/exec` Runner interface, Real, DryRun, Fake (+ tests)
- [ ] T004 [P] `internal/output` text/JSON printer (+ tests)
- [ ] T005 `internal/cli/errors.go` typed errors and `ExitCode` mapping (0/1/2/64) (+ tests)

## Phase 3: User Story 1 — command tree (P1)

- [ ] T006 [US1] Golden test for `aibox --help` and not-implemented behavior in `internal/cli/root_test.go`
- [ ] T007 [US1] `internal/cli/root.go` root command, global flags, logger setup
- [ ] T008 [US1] `internal/cli/notimpl.go` stubs for up, down, models, smoke, bench, report, cost (exit 2)
- [ ] T009 [US1] `internal/cli/version.go` (`aibox version`, text/JSON)
- [ ] T010 [US1] `cmd/aibox/main.go` wiring and exit codes

## Phase 4: User Story 3 — profiles (P1)

- [ ] T011 [US3] `schemas/profile.schema.json` + `schemas/embed.go`
- [ ] T012 [P] [US3] `profiles/local.yaml`, `profiles/aws-dev.yaml`, `profiles/aws-bench.yaml`
- [ ] T013 [US3] Tests first: `internal/profile/load_test.go` with valid profiles and invalid fixtures in `internal/profile/testdata/`
- [ ] T014 [US3] `internal/profile` types, resolution (name/path, repo root, `AIBOX_PROFILES_DIR`) and validation with JSON paths

## Phase 5: User Story 2 — local doctor (P1)

- [ ] T015 [US2] Tests first: `internal/doctor/local_test.go` (all pass, daemon down, tool too old, low memory) + golden JSON
- [ ] T016 [US2] `internal/doctor/doctor.go` report model; `tools.go` version parsing; `local.go` checks via Runner
- [ ] T017 [US2] `internal/cli/doctor.go` command (text table / JSON, exit 1 on fail)

## Phase 6: User Story 4 — AWS doctor (P2)

- [ ] T018 [US4] Tests first: `internal/doctor/aws_test.go` with a fake AWS client (no creds, quota low, type not offered, all good)
- [ ] T019 [US4] `internal/doctor/aws.go` checks + `internal/doctor/awsclient.go` SDK-backed client (read-only)

## Phase 7: User Story 5 — Makefile wrappers (P2)

- [ ] T020 [US5] Makefile `build` and wrappers `up down models-sync smoke bench report cost` with `PROFILE`

## Phase 8: Polish

- [ ] T021 [P] `docs/adr/0008-cli-in-go-with-cobra.md`; ADR index
- [ ] T022 [P] CI job `cli` (doctor --profile local, `up` exits 2) added to `ci-gate`
- [ ] T023 [P] CONTRIBUTING/README: CLI usage section
- [ ] T024 Verify `go test ./... -race -cover` (≥ 80% profile/doctor), `make lint`, definition of done
- [ ] T025 Update docs/agent/PROGRESS.md

## Dependencies

Setup → Foundational → US1 → US3 → US2 → US4 → US5 → Polish. Tests precede implementation
within each story.
