# Implementation Plan: Inference runtimes for catalog models

**Branch**: `008-inference-runtime` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

## Summary

KServe v0.21.0 (`LLMInferenceService`, workload only) and cert-manager v1.21.3 become Argo CD
components of every profile. `internal/serving` renders one LLMInferenceService per enabled,
served catalog model (vLLM, SGLang or the simulator) with flags, resources, placement,
probes, a warm-up hook, the drain settings and the weights loading method;
`aibox render serving` writes `platform/serving/overlays/<profile>/models.yaml` (committed,
drift-tested). The bootstrap chart renders the `aibox-run` ConfigMap (weights bucket, run id).
`metrics/runtime-metrics.yaml` and `test/contract` pin the metric contract; e2e-local runs it
against the simulator served by KServe.

## Technical Context

**Language/Version**: Go 1.27.3; Python 3.14 for the warm-up hook and `mirror.py fetch`.

**Primary Dependencies**: KServe charts `oci://ghcr.io/kserve/charts/kserve-llmisvc-{crd,resources}`
v0.21.0 (controller `kserve/llmisvc-controller:v0.21.0@sha256:be2261cc…`), cert-manager chart
v1.21.3, `vllm/vllm-openai:v0.33.0@sha256:d825baf9…`, `lmsysorg/sglang:v0.5.23@sha256:f832b5c3…`,
`ghcr.io/llm-d/llm-d-inference-sim:v1.1.0@sha256:012cf6aa…`, `github.com/prometheus/common/expfmt`.

**Testing**: golden + drift tests for rendering; validation tests (TP vs GPUs, unsupported
combinations); metric contract fixtures; kubeconform with the LLMInferenceService CRD schema;
e2e-local (KServe serves the simulator, smoke, live contract test).

**Constraints**: no GPUs in CI; runtime flags verified against sources at the pinned versions.

## Constitution Check

| Principle | Status | Evidence |
| --- | --- | --- |
| I | Pass | ADR-0002 (Accepted) records KServe vs raw llm-d. |
| II | Pass | Charts and images pinned (digests); flags from sources. |
| III | Pass | Local profile serves through the same templates (simulator). |
| IV | Pass | No new cloud resources; GPUs only on demand (Karpenter). |
| V | Pass | Everything delivered by Argo CD; generated files reviewed in PRs. |
| VI | Pass | Golden/drift/validation/contract tests; e2e. |
| VII | Pass | `/metrics` contract and mapping file for 010/012. |
| VIII | Pass | Non-root pods (KServe template), read-only bucket role, no secrets in git. |
| IX | Pass | Upstream images only. |
| X | Pass | ADR-0002, runbook `model-load-failures.md`, docs. |

## Project Structure

```text
internal/serving/{render.go,runtimes.go,warmup.py}   # + tests, testdata (golden)
internal/cli/render.go                                # `aibox render serving`
internal/models/scripts/mirror.py                     # + fetch mode (nvme-cache)
schemas/catalog.schema.json, models/catalog.yaml      # shutdownGracePeriodSeconds, startupTimeoutSeconds
metrics/runtime-metrics.yaml
test/contract/runtime_metrics_test.go (+ testdata)
platform/cert-manager/base, platform/kserve/base
platform/serving/overlays/{local,aws-dev,aws-bench}/   # models.yaml generated
platform/bootstrap/templates/run-config.yaml           # aibox-run ConfigMap
docs/adr/0002-kserve-llminferenceservice.md, docs/runbooks/model-load-failures.md
```

## Complexity Tracking

No violations.
