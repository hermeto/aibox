# Tasks: Cost guardrails

**Input**: Design documents from `/specs/005-cost-guardrails/`

## Phase 1: Setup

- [X] T001 Add aws-sdk-go-v2 `service/elasticloadbalancingv2` and `service/pricing`
- [X] T002 [P] `data/pricing/aws-us-east-1.yaml` (schema-checked by a unit test)

## Phase 2: Foundational

- [X] T003 [P] `internal/cost/pricing.go` loader + staleness/verified checks (+ tests)
- [X] T004 [P] `infra/terraform/aws/guardrails` stack + `buildspec.yml` + mocked `tofu test`

## Phase 3: US1 — budget and TTL required (P1)

- [X] T005 [US1] Tests first: `TestUpAWSRequiresBudget`, `TestUpAWSRequiresEmail`, guardrail tfvars rendering
- [X] T006 [US1] `internal/platform/aws.go`: budget/e-mail/TTL inputs, guardrails stack apply before main; CLI flags `--budget`, `--notify-email`, `--ttl`

## Phase 4: US2 — TTL teardown and extension (P1)

- [X] T007 [US2] Tests first: `internal/teardown` (expiry = start + TTL; extend moves it; > 24 h refused; no active run)
- [X] T008 [US2] `internal/teardown/ttl.go`, `aibox ttl extend|show` (`internal/cli/ttl.go`, `platform.AWS.ExtendTTL`)

## Phase 5: US3 — orphan check (P1)

- [X] T009 [US3] Fixtures in `internal/cost/testdata/` (recorded EC2/ELBv2 responses) and tests first
- [X] T010 [US3] `internal/cost/orphans.go` + SDK clients; `down` = destroy main → destroy guardrails → orphans; `--orphans-only`

## Phase 6: US4 — `aibox cost` (P1)

- [X] T011 [US4] Tests first: cost math (spot vs on-demand, unknown price, fixed items)
- [X] T012 [US4] `internal/cost/cost.go`, `internal/cli/cost.go` (replaces stub)

## Phase 7: US5 — estimate in doctor (P2)

- [X] T013 [US5] `internal/cost/estimate.go` + doctor check `cost.estimate` (+ tests)

## Phase 8: Polish

- [X] T014 [P] `hack/pricing/main.go` refresh tool (not in CI)
- [X] T015 [P] ADR-0010; runbook `docs/runbooks/cost-guardrails.md`
- [X] T016 Verify DoD (`go test ./internal/cost/... ./internal/teardown/...` — teardown logic lives in `internal/platform`, `tofu validate && tofu test`); PROGRESS

## Analysis log

`/speckit-analyze` pass 1 (2026-11-05): 0 CRITICAL, 1 HIGH — the DoD names `./internal/teardown/...`, which the plan does not create (teardown orchestration lives in `internal/platform`, the executor in Terraform) → added `internal/teardown` as a small package holding the schedule/expiry logic so the DoD command is meaningful (T007/T008 updated). Coverage FR 9/9, SC 4/4. Pass 2: clean.

## Convergence log

`/speckit-converge` pass 1 (2026-11-05): 2 findings, fixed in this pass.

- F1 `contradicts` (HIGH, FR-002): the budget filter used an escaped interpolation in HCL,
  which renders the literal text instead of the run ID (caught by the guardrails test) →
  rewritten with `format()`.
- F2 `missing` (HIGH, FR-003): the teardown job could not drain the cluster (no EKS access)
  → main stack `access_entries` for the teardown role (`teardown_role_arn`), tested in
  `validation.tftest.hcl`.

Pass 2: 0 findings. Evidence: `go test ./internal/cost/... ./internal/teardown/...` (94.7%,
100%), `TestUpAWS*`/`TestDownAWS*`/`TestTTLExtendAndCost`, guardrails `tofu test` 3/3, main
14/14, Trivy config clean.
