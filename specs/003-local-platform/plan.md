# Implementation Plan: `local` profile on kind with GitOps bootstrap

**Branch**: `003-local-platform` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/003-local-platform/spec.md`

## Summary

`aibox up --profile local` runs doctor, creates/reuses the kind cluster from
`infra/local/kind-config.yaml`, installs Argo CD with Helm (chart `argo-cd` 10.12.1, app
v3.6.1), waits for Argo CD deployments via client-go, applies the root Application
(`platform/bootstrap` Helm chart, `values-local.yaml`, parameters `repoURL`/`targetRevision`)
with server-side apply, and waits for every Application to be Synced/Healthy using the
dynamic client. The `serving` component deploys `sim-llama-8b` (llm-d-inference-sim v1.1.0,
multi-arch, by digest) behind NodePort 30080. `aibox smoke` checks `/v1/models`, a chat
completion and required metric names. `aibox down` deletes the cluster. `--from-worktree`
renders overlays with `kubectl kustomize --enable-helm` and applies them directly. CI job
`e2e-local` (Go test `test/e2e`, build tag `e2e`) runs up → up (idempotency) → smoke → down.
ADR-0009 records GitOps with Argo CD app-of-apps.

## Technical Context

**Language/Version**: Go 1.27.3.

**Primary Dependencies**: `k8s.io/client-go` v0.37.2, `k8s.io/apimachinery` v0.37.2
(dynamic client, server-side apply, clientcmd); external tools via `internal/exec`: kind
v0.33.0, helm v4.3.1, kubectl v1.37.2 (CI pins). Argo CD chart 10.12.1 (app v3.6.1),
llm-d-inference-sim v1.1.0
(`ghcr.io/llm-d/llm-d-inference-sim@sha256:012cf6aa414cfbb910743274458dd62c0c0a9d469eb2ba2b28fd5b963516447d`),
kind node `kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5`.

**Storage**: N/A (kubeconfig written by kind to the user's default kubeconfig, context
`kind-aibox-local`).

**Testing**: unit tests with `exec.Fake` and client-go fake dynamic client (`up`/`down`/
`smoke` orchestration, Application status evaluation, repo URL normalization); `httptest`
server for smoke checks; e2e test `test/e2e/local_test.go` (`//go:build e2e`) in CI.

**Target Platform**: Linux and macOS hosts with Docker; amd64 and arm64.

**Project Type**: CLI + GitOps manifests.

**Performance Goals**: bring-up ≤ 10 minutes on a 4-CPU/8-GB runner (SC-002).

**Constraints**: no cloud credentials; images multi-arch; host ports bound to 127.0.0.1.

**Scale/Scope**: 1 control-plane + 1 worker kind cluster; 1 simulated model.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
| --- | --- | --- |
| I. Evidence | Pass | ADR-0009 with criteria. |
| II. Reproducible | Pass | Chart version, node image and simulator image pinned by version + digest; Argo CD image digest pinned from the first CI run (see research R3). |
| III. Local-first | Pass | This feature *is* the local profile; `e2e-local` on every PR. |
| IV. Zero orphans | Pass | `down` deletes the cluster; no cloud resources. |
| V. GitOps | Pass | Everything under `platform/` delivered by Argo CD; imperative steps limited to kind + Argo CD install + root Application, all idempotent. |
| VI. Tests | Pass | Unit tests for orchestration with fakes; e2e in CI. |
| VII. Observability | Pass | Simulator exposes `/metrics`; smoke asserts metric names. |
| VIII. Secure | Pass | Restricted pod security for the simulator; NodePorts bound to 127.0.0.1; Argo CD admin secret never printed (UI hint uses `kubectl get secret`). |
| IX. Simplicity | Pass | NodePort instead of port-forward processes; Helm app-of-apps instead of ApplicationSets. |
| X. OSS | Pass | 5-minute quickstart in README. |

## Project Structure

### Documentation (this feature)

```text
specs/003-local-platform/
├── plan.md research.md data-model.md quickstart.md
├── contracts/{up-down.md,smoke-report.md}
├── checklists/{requirements.md,quality.md}
└── tasks.md
```

### Source Code (repository root)

```text
infra/local/kind-config.yaml               # pinned node image, port mappings, worker label
platform/bootstrap/                        # Helm chart: app-of-apps
├── Chart.yaml  values.yaml  values-local.yaml
├── templates/applications.yaml
└── argocd/values-local.yaml               # Helm values for the Argo CD install (CLI)
platform/serving/base/                     # simulator Deployment + Service (sim-llama-8b)
platform/serving/overlays/local/           # NodePort 30080, latency flags
internal/kube/                             # client-go helpers: config, SSA apply, wait (Deployments, Applications)
internal/platform/                         # up/down orchestration (local), git source detection
internal/smoke/                            # OpenAI-compatible smoke checks
internal/cli/{up.go,down.go,smoke.go}      # commands (replace stubs)
test/e2e/local_test.go                     # //go:build e2e
.github/workflows/ci.yml                   # job e2e-local, added to ci-gate
docs/adr/0009-gitops-with-argocd-app-of-apps.md
```

**Structure Decision**: Appendix A §8; Kubernetes access isolated in `internal/kube` so
orchestration in `internal/platform` is unit-testable with fakes.

## Complexity Tracking

No violations.
