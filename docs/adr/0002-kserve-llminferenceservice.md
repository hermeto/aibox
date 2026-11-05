# ADR-0002: KServe LLMInferenceService as the per-model serving resource

- **Status**: Accepted
- **Date**: 2026-11-05
- **Deciders**: AIBox maintainers (proposed by the agent in feature 008)
- **Decided by**: feature 008

## Context and problem statement

Every catalog model needs a model server (vLLM by default, SGLang as an alternative) with
pinned images, catalog flags, GPU placement, probes and drain behaviour, the same way in every
profile. Routing (feature 011: InferencePool + llm-d Endpoint Picker) and autoscaling
(feature 012: HPA/KEDA and the Workload Variant Autoscaler) build on top of it. The product
spec (FR-GW-4) names KServe `LLMInferenceService` as the high-level per-model resource and asks
this ADR to settle it against the raw llm-d Helm charts.

## Decision drivers

- Works on kind without GPUs (simulator) through the same manifests as on EKS.
- One resource per model that later features configure rather than replace.
- Pinned, verifiable upstream APIs (fast-moving projects).
- Smallest footprint in the local bring-up budget (10 minutes).

## Considered options

1. **KServe `LLMInferenceService`** (`serving.kserve.io/v1alpha2`, KServe v0.21.0): the
   controller and CRDs from two OCI charts, cert-manager for webhooks; scheduler/InferencePool and scaling
   (HPA, KEDA, WVA) are fields of the same resource.
2. **Raw llm-d Helm charts**: `llm-d-modelservice` for the servers and the Gateway API Inference
   Extension `inferencepool` chart for the Endpoint Picker; no controller, one Helm release per
   model, values that change with each llm-d release.
3. **AIBox-rendered Deployments**: plain manifests from the catalog; no controller, but AIBox
   would re-implement what options 1 and 2 provide for routing and scaling.

## Decision outcome

Chosen option: "KServe `LLMInferenceService`", because it is the product's stated direction and
the KServe v0.21.0 controller covers the next features (`spec.router.scheduler` for 011,
`spec.scaling` with HPA/KEDA and WVA for 012) without another installation. Its controller only
watches Gateway API, InferencePool, KEDA and LeaderWorkerSet types when their CRDs exist
(`pkg/controller/v1alpha2/llmisvc/controller.go`), so the workload part runs before the gateway
exists, and with `storageInitializer.enabled: false` it leaves weight loading to AIBox
(`workload_storage.go`).

AIBox renders one LLMInferenceService per enabled, served catalog model
(`aibox render serving`, committed per profile, drift-tested) that overrides the `main`
container of KServe's default template (image, command, flags, probes, warm-up hook, drain,
resources, placement) and keeps its volumes, security context and `preStop` delay. `model.uri`
records the provenance only.

### Consequences

- Good: routing and autoscaling become configuration of the same resource; one CRD to learn.
- Good: KServe's default template stays in charge of shared details (shared memory, TLS
  certificates when enabled, `preStop`), so upgrades bring fixes.
- Bad: cert-manager and the KServe controller are added to every profile (about one minute of
  local bring-up).
- Bad: the merge with KServe's template is a strategic merge patch; a KServe upgrade that
  changes the template needs the golden tests and e2e to be re-checked.
- Fallback: the raw llm-d path (option 2) needs only the serving overlays to change, because
  the catalog and the rendering inputs stay the same.

## Links

- Spec: `specs/008-inference-runtime/spec.md` (Clarifications), research notes with source paths.
- Product spec: FR-GW-4, FR-RT-1…5.
