# Implementation Plan: Cost guardrails

**Branch**: `005-cost-guardrails` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/005-cost-guardrails/spec.md`

## Summary

A new Terraform stack `infra/terraform/aws/guardrails` (budget + SNS alert topic, EventBridge
Scheduler one-time schedule, CodeBuild teardown project and its IAM roles) is applied before
and destroyed after the main stack. `aibox up` requires `--budget`/`BUDGET_USD` and an alert
e-mail and renders guardrail inputs (budget, e-mail, TTL expiry, the main stack's inputs and
backend so the teardown can destroy it). `aibox ttl extend` re-applies guardrails with a later
expiry. `aibox down` destroys main → guardrails → runs the orphan checker
(`internal/cost/orphans.go`, read-only EC2/ELBv2 describes behind interfaces, fixtures =
recorded API responses). `aibox cost` (`internal/cost/cost.go`) prices tagged instances with
`data/pricing/aws-<region>.yaml`; doctor adds a worst-case estimate. `hack/pricing` refreshes
the table from the Price List API (never in CI). ADR-0010.

## Technical Context

**Language/Version**: Go 1.27.3; HCL (OpenTofu 1.13.8).

**Primary Dependencies**: aws-sdk-go-v2 `service/elasticloadbalancingv2`, `service/pricing`
(refresh tool only); existing `service/ec2`. Terraform `aws_budgets_budget`, `aws_sns_topic`,
`aws_scheduler_schedule`, `aws_codebuild_project`, `aws_iam_role`.

**Storage**: price table YAML; run state `.aibox/<profile>/run.json` (+ `expiresAt`).

**Testing**: Go unit tests with fakes/fixtures (`internal/cost`, `internal/platform`),
`tofu test` with mocked providers for the guardrails stack.

**Target Platform**: AWS (teardown in CodeBuild, `amazonlinux-x86_64-standard:5.0` image).

**Project Type**: CLI + IaC.

**Performance Goals**: teardown completes within the CodeBuild timeout (90 min).

**Constraints**: every check/cost call read-only; the agent never applies guardrails.

**Scale/Scope**: one budget, one schedule, one CodeBuild project per run.

## Constitution Check

| Principle | Status | Evidence |
| --- | --- | --- |
| I. Evidence | Pass | ADR-0010 with the executor trade-offs. |
| II. Reproducible | Pass | OpenTofu version pinned in the buildspec and checksum-verified; price table versioned with date and source. |
| III. Local-first | Pass | Local profile unaffected; all logic unit-tested offline. |
| IV. Zero orphans | Pass | This feature implements it: budget, TTL, orphan checker. |
| V. Declarative | Pass | Guardrails in Terraform. |
| VI. Tests | Pass | Fixture-driven orphan tests; cost math tests; mocked Terraform tests. |
| VII. Observability | Pass | Cost report; alerts by e-mail. |
| VIII. Secure | Pass | Teardown role assumable only by CodeBuild in this account; read-only checker. Broad delete permissions are an accepted, documented risk (ADR-0010). |
| IX. Simplicity | Pass | No Lambda, no custom images. |
| X. OSS | Pass | Runbook `docs/runbooks/cost-guardrails.md`. |

## Project Structure

```text
infra/terraform/aws/guardrails/{versions,variables,main,outputs}.tf, buildspec.yml, tests/guardrails.tftest.hcl
internal/cost/{pricing.go,cost.go,orphans.go,estimate.go} (+ tests, testdata/*.json)
internal/awsapi/clients.go         # EC2/ELBv2 clients for cost + orphans
internal/platform/aws.go           # budget/TTL/guardrails stack, down → orphan check, ttl extend
internal/cli/{up.go,cost.go,ttl.go}
internal/doctor/estimate.go        # worst-case estimate check
data/pricing/aws-us-east-1.yaml
hack/pricing/main.go               # refresh tool (not in CI)
docs/runbooks/cost-guardrails.md, docs/adr/0010-mandatory-budget-and-ttl.md
```

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
| --- | --- | --- |
| Teardown role with broad delete permissions | Destroying VPC/EKS/IAM/KMS/S3 needs delete rights across services | Tag-conditioned IAM is not supported by many delete actions; scoping to the account + CodeBuild trust is the practical bound |
