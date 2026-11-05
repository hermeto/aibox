# Tasks: AWS infrastructure as code (VPC, EKS, IAM, storage)

**Input**: Design documents from `/specs/004-aws-infrastructure/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: mocked Terraform tests and Go fake-runner tests (the agent never applies).

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [ ] T001 `infra/terraform/aws/{versions,providers,backend}.tf` (pinned providers, default tags, partial S3 backend)
- [ ] T002 [P] `.tflint.hcl` (AWS ruleset v0.50.0) and `.gitignore` entries (`.aibox/`, `.terraform/`)
- [ ] T003 [P] AWS profiles: `kubernetesVersion: "1.35"`

## Phase 2: Foundational

- [ ] T004 `variables.tf` with validations (data-model.md)
- [ ] T005 [P] `modules/storage` (KMS CMK, bucket, versioning, SSE-KMS + bucket key, public access block, TLS-only policy)
- [ ] T006 [P] `modules/iam` (Pod Identity roles + associations, scoped policies)

## Phase 3: User Story 1 — create the environment (P1)

- [ ] T007 [US1] `main.tf`: VPC, EKS (system node group, addons, endpoint CIDRs, logs), Karpenter submodule, storage, iam; `outputs.tf`
- [ ] T008 [US1] Tests first: `internal/platform/aws_test.go` (`TestUpAWS*`, `TestDownAWS*`: doctor gate, state bucket, tfvars rendering, init/plan/apply/output sequence, confirmation, dry-run)
- [ ] T009 [US1] `internal/platform/gitops.go` (shared Argo CD + root app bootstrap) and `internal/platform/aws.go` (run ID/owner resolution, operator /32 detection, state-bucket existence check with the bootstrap hint, outputs file, kubeconfig context)
- [ ] T010 [US1] CLI: aws path in `up`/`down` (`--admin-cidr`, `--owner`), confirmation prompt
- [ ] T011 [P] [US1] `platform/bootstrap/values-aws-*.yaml`, `platform/bootstrap/argocd/values-aws-*.yaml`

## Phase 4: User Story 2 — offline validation (P1)

- [ ] T012 [US2] `tests/root.tftest.hcl` (override_module; wiring, tags, endpoint CIDRs), `tests/storage.tftest.hcl`, `tests/iam.tftest.hcl`, `tests/validation.tftest.hcl` (expect_failures)
- [ ] T013 [US2] CI job `terraform` (setup-opentofu pinned; fmt, init -backend=false, validate, test, tflint) in `ci-gate`; commit `.terraform.lock.hcl`

## Phase 5: User Story 3 — tags (P1)

- [ ] T014 [US3] Tag assertions in tests (default tags; representative resources)

## Phase 6: User Story 4 — state bootstrap (P2)

- [ ] T015 [US4] `bootstrap/` stack + `bootstrap/tests/bootstrap.tftest.hcl`

## Phase 7: Polish

- [ ] T016 [P] `docs/runbooks/aws-first-run.md` (quotas, region, bootstrap, first run, teardown)
- [ ] T017 [P] ADR-0001 EKS + Karpenter substrate; ADR index
- [ ] T018 Verify DoD (`tofu validate && tofu test`, tflint, trivy config, `go test -run 'TestUp.*AWS'`); update PROGRESS.md

## Analysis log

`/speckit-analyze` pass 1 (2026-11-05): 0 CRITICAL, 1 HIGH — edge case "state bucket missing → stop with bootstrap command" and the run-ID/owner clarification were not explicit in any task → added to T009. Coverage FR 8/8, SC 4/4 (SC-003 human-verified via T016). Pass 2: clean.

## Dependencies

Setup → Foundational → US1 → US2 → US3 → US4 → Polish.
