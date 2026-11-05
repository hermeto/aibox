# Implementation Plan: AWS infrastructure as code (VPC, EKS, IAM, storage)

**Branch**: `004-aws-infrastructure` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/004-aws-infrastructure/spec.md`

## Summary

One Terraform root in `infra/terraform/aws/` (OpenTofu-tested, Terraform-compatible) wires
`terraform-aws-modules/vpc` v6.7.3 (3 AZs, single NAT), `terraform-aws-modules/eks` v21.29.0
(Kubernetes 1.35, managed on-demand `m7i.xlarge` system node group, Pod Identity agent,
public endpoint limited to the operator's /32) and its `karpenter` submodule, plus two
AIBox modules: `modules/storage` (KMS-encrypted, versioned, TLS-only, non-public weights
bucket) and `modules/iam` (Pod Identity roles + associations for External Secrets, the OTel
collector, model sync and model servers). Provider `default_tags` apply `project`, `run-id`,
`owner`, `profile`. A separate `bootstrap/` stack creates the state bucket (S3 native
locking). `terraform test` files with `mock_provider` and `override_module` cover
validation, tags, encryption and IAM scoping. The CLI (`internal/platform/aws.go`) renders
`.aibox/<profile>/terraform.tfvars.json`, runs `init` (S3 backend), `plan`, confirmation,
`apply`, `output -json`, writes a kubeconfig context and reuses the GitOps bootstrap;
`down` runs `destroy`. CI job `terraform` runs fmt/validate/tflint/trivy/test. ADR-0001.

## Technical Context

**Language/Version**: HCL (OpenTofu 1.13.8 in CI; Terraform 1.17.1 compatible), Go 1.27.3.

**Primary Dependencies**: `hashicorp/aws` 6.72.0, `hashicorp/tls`, `hashicorp/time` (EKS
module), modules above (git sources pinned to tags); tflint v0.65.0 + AWS ruleset v0.50.0;
Trivy 0.76.1 (existing CI job scans HCL).

**Storage**: S3 remote state (`use_lockfile = true`); weights bucket (SSE-KMS).

**Testing**: `tofu test` / `terraform test` with `mock_provider "aws"` and `override_module`;
Go unit tests with `exec.Fake` (`TestUp*AWS*`, `TestDown*AWS*`).

**Target Platform**: AWS, one region per run (default `us-east-1`).

**Project Type**: Infrastructure as code + CLI orchestration.

**Performance Goals**: bring-up to "cluster reachable" ≤ 20 min (human-verified).

**Constraints**: the agent never runs apply/destroy; no credentials in CI; no account IDs in
git; HIGH/CRITICAL Trivy config findings fail CI.

**Scale/Scope**: 1 VPC, 1 EKS cluster, 1 system node group, ~5 Pod Identity roles, 1 bucket.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
| --- | --- | --- |
| I. Evidence | Pass | ADR-0001 (EKS + Karpenter substrate) with criteria. |
| II. Reproducible | Pass | Provider versions exact + `.terraform.lock.hcl`; modules pinned to tags. |
| III. Local-first | Pass | Local profile unaffected; Terraform validated in CI without credentials. |
| IV. Zero orphans / no unattended spend | Pass | Apply/destroy only on explicit human action (`--yes` or prompt); everything tagged for 005. |
| V. Declarative | Pass | Infra in HCL; GitOps bootstrap reused. |
| VI. Tests | Pass | Mocked Terraform tests; Go tests for the command sequence. |
| VII. Observability | Pass | Cluster control-plane logs enabled (api, audit, authenticator). |
| VIII. Secure | Pass | Least-privilege Pod Identity; KMS; TLS-only bucket; endpoint restricted to /32; IMDSv2 required. |
| IX. Simplicity | Pass | Community modules; single NAT; one root with tfvars rendered by the CLI. |
| X. OSS | Pass | Runbook `docs/runbooks/aws-first-run.md`. |

## Project Structure

### Documentation (this feature)

```text
specs/004-aws-infrastructure/
├── plan.md research.md data-model.md quickstart.md
├── contracts/{terraform-io.md,cli-aws.md}
├── checklists/{requirements.md,quality.md}
└── tasks.md
```

### Source Code (repository root)

```text
infra/terraform/aws/
├── versions.tf providers.tf variables.tf main.tf outputs.tf backend.tf .tflint.hcl
├── .terraform.lock.hcl
├── modules/storage/{main,variables,outputs,versions}.tf
├── modules/iam/{main,variables,outputs,versions}.tf
├── tests/{root,storage,iam,validation}.tftest.hcl
└── bootstrap/{main,variables,outputs,versions}.tf + tests/bootstrap.tftest.hcl
internal/platform/aws.go          # AWS up/down orchestration (fake-runner tested)
internal/platform/gitops.go       # shared Argo CD + root Application bootstrap
internal/cli/up.go                # aws path (flags --admin-cidr, --owner)
platform/bootstrap/values-aws-dev.yaml, values-aws-bench.yaml, argocd/values-aws-*.yaml
docs/runbooks/aws-first-run.md
docs/adr/0001-eks-karpenter-substrate.md
.github/workflows/ci.yml          # job terraform (+ ci-gate)
```

**Structure Decision**: one root module + profile inputs (rendered tfvars), not one root per
profile: profiles differ only in inputs.

## Complexity Tracking

No violations.
