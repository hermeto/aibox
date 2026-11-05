# Implementation Plan: Just-in-time GPU nodes with Karpenter and GPU telemetry

**Branch**: `006-gpu-node-provisioning` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

## Summary

`internal/gpu` renders Karpenter `EC2NodeClass` + `NodePool` objects from a profile's GPU
classes (`aibox render nodepools`), committed per aws overlay and drift-checked. Karpenter
v1.15.1 is installed by Argo CD from its OCI chart (`platform/karpenter`, wave −1); the
`platform/gpu` component (wave 0) carries the node pools and the DCGM exporter (chart 4.8.3).
kind gets a fake GPU worker; the simulator moves onto it. `make validate-manifests` validates
Karpenter objects with schemas generated from the pinned CRDs; `hack/check-image-digests.sh`
enforces image digests in CI. Runbooks: gpu-nodes, spot-interruption.

## Technical Context

**Language/Version**: Go 1.27.3; YAML (Kustomize, Helm values).

**Primary Dependencies**: Karpenter 1.15.1 (chart `oci://public.ecr.aws/karpenter/karpenter`,
controller `public.ecr.aws/karpenter/controller:1.15.1@sha256:8314ff9c…`), Bottlerocket
v1.68.1 AMI alias, DCGM exporter chart 4.8.3 (`nvcr.io/nvidia/k8s/dcgm-exporter:4.6.0-4.8.3-distroless`),
`sigs.k8s.io/kustomize/api` (krusty) for scheduling tests.

**Testing**: golden/drift test for rendered pools; krusty-based scheduling test (serving
tolerations/selectors vs pool taints/labels); kubeconform with Karpenter CRD schemas.

**Constraints**: no GPU in CI; Karpenter objects validated offline.

## Constitution Check

| Principle | Status | Evidence |
| --- | --- | --- |
| I | Pass | ADR-0001 extended (AMI/consolidation decisions recorded in research). |
| II | Pass | Chart versions, controller digest, AMI alias version pinned; digest check in CI. |
| III | Pass | Fake GPU node on kind; scheduling test runs locally. |
| IV | Pass | Scale to zero; per-pool limits cap spend. |
| V | Pass | Pools generated into git, delivered by Argo CD. |
| VI | Pass | Drift, scheduling and schema tests. |
| VII | Pass | DCGM exporter (scraped in 010). |
| VIII | Pass | IMDSv2, Pod Identity for Karpenter (004). |
| IX | Pass | No GPU Operator. |
| X | Pass | Runbooks. |

## Project Structure

```text
internal/gpu/{render.go,instances.go} (+ tests, testdata)
internal/cli/render.go                       # aibox render nodepools
platform/karpenter/overlays/{aws-dev,aws-bench}/kustomization.yaml
platform/gpu/base/{kustomization.yaml}       # DCGM exporter (helmCharts)
platform/gpu/overlays/{aws-dev,aws-bench}/{kustomization.yaml,nodepools.yaml}
platform/bootstrap/values-aws-*.yaml          # karpenter (−1), gpu (0)
infra/local/kind-config.yaml                  # worker2 = fake GPU
platform/serving/base/sim-llama-8b.yaml       # toleration + nodeSelector
hack/gen-crd-schemas.sh, hack/schemas/karpenter.*  hack/check-image-digests.sh
docs/runbooks/{gpu-nodes.md,spot-interruption.md}
```

## Complexity Tracking

No violations.
