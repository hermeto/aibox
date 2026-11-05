# Tasks: AWS infrastructure as code (VPC, EKS, IAM, storage)

**Input**: Design documents from `/specs/004-aws-infrastructure/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: mocked Terraform tests and Go fake-runner tests (the agent never applies).

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [X] T001 `infra/terraform/aws/{versions,providers,backend}.tf` (pinned providers, default tags, partial S3 backend)
- [X] T002 [P] `.tflint.hcl` (AWS ruleset v0.50.0) and `.gitignore` entries (`.aibox/`, `.terraform/`)
- [X] T003 [P] AWS profiles: `kubernetesVersion: "1.35"`

## Phase 2: Foundational

- [X] T004 `variables.tf` with validations (data-model.md)
- [X] T005 [P] `modules/storage` (KMS CMK, bucket, versioning, SSE-KMS + bucket key, public access block, TLS-only policy)
- [X] T006 [P] `modules/iam` (Pod Identity roles + associations, scoped policies)

## Phase 3: User Story 1 — create the environment (P1)

- [X] T007 [US1] `main.tf`: VPC, EKS (system node group, addons, endpoint CIDRs, logs), Karpenter submodule, storage, iam; `outputs.tf`
- [X] T008 [US1] Tests first: `internal/platform/aws_test.go` (`TestUpAWS*`, `TestDownAWS*`: doctor gate, state bucket, tfvars rendering, init/plan/apply/output sequence, confirmation, dry-run)
- [X] T009 [US1] `internal/platform/gitops.go` (shared Argo CD + root app bootstrap) and `internal/platform/aws.go` (run ID/owner resolution, operator /32 detection, state-bucket existence check with the bootstrap hint, outputs file, kubeconfig context)
- [X] T010 [US1] CLI: aws path in `up`/`down` (`--admin-cidr`, `--owner`), confirmation prompt
- [X] T011 [P] [US1] `platform/bootstrap/values-aws-*.yaml`, `platform/bootstrap/argocd/values-aws-*.yaml`

## Phase 4: User Story 2 — offline validation (P1)

- [X] T012 [US2] `tests/root.tftest.hcl` (override_module; wiring, tags, endpoint CIDRs), `tests/storage.tftest.hcl`, `tests/iam.tftest.hcl`, `tests/validation.tftest.hcl` (expect_failures)
- [X] T013 [US2] CI job `terraform` (setup-opentofu pinned; fmt, init -backend=false, validate, test, tflint) in `ci-gate`; commit `.terraform.lock.hcl`

## Phase 5: User Story 3 — tags (P1)

- [X] T014 [US3] Tag assertions in tests (default tags; representative resources)

## Phase 6: User Story 4 — state bootstrap (P2)

- [X] T015 [US4] `bootstrap/` stack + `bootstrap/tests/bootstrap.tftest.hcl`

## Phase 7: Polish

- [X] T016 [P] `docs/runbooks/aws-first-run.md` (quotas, region, bootstrap, first run, teardown)
- [X] T017 [P] ADR-0001 EKS + Karpenter substrate; ADR index
- [X] T018 Verify DoD (`tofu validate && tofu test`, tflint, trivy config, `go test -run 'TestUp.*AWS'`); update PROGRESS.md

## Analysis log

`/speckit-analyze` pass 1 (2026-11-05): 0 CRITICAL, 1 HIGH — edge case "state bucket missing → stop with bootstrap command" and the run-ID/owner clarification were not explicit in any task → added to T009. Coverage FR 8/8, SC 4/4 (SC-003 human-verified via T016). Pass 2: clean.

## Dependencies

Setup → Foundational → US1 → US2 → US3 → US4 → Polish.

## Convergence log

`/speckit-converge` pass 1 (2026-11-05): 2 findings, both fixed in this pass.

- F1 `contradicts` (HIGH, research R1): git module sources did not make local validation
  possible (the EKS module pulls `terraform-aws-modules/kms` from the registry) → switched to
  registry sources with exact versions; local checks use a throwaway registry shim.
- F2 `partial` (MEDIUM, US2): OpenTofu evaluates overridden module locals, so
  `override_module` tests failed under `tofu test` → tests plan/apply the real modules with
  mocked providers (ARN-shaped mock defaults), and `enable_irsa = false` (Pod Identity only).
Pass 2: 0 findings. Evidence: `make terraform-check` (root 12/12, bootstrap 1/1 with
OpenTofu 1.13.8; root also 12/12 with Terraform 1.17.1 before F2), `go test -run 'TestUp.*AWS|TestDown.*AWS'`,
Trivy config clean with two scoped, expiring risk acceptances (`.trivyignore.yaml`).
