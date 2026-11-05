# Feature Specification: Inference runtimes for catalog models

**Feature Branch**: `008-inference-runtime`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "Inference runtimes for catalog models. AIBox standardizes on vLLM, keeps SGLang as a comparable alternative, and exposes the same OpenAI-compatible API and metrics for both so experiments can swap them. Each catalog model becomes a KServe LLMInferenceService (or equivalent per ADR-0002) rendered from the catalog with a pinned runtime image, flags, resources, GPU class placement and probes; readiness only after weights are loaded and a warm-up request succeeds; graceful drain on shutdown; a metric contract test; SGLang selectable per model; FP8 weights and FP8 KV cache as catalog options (playbook §6, feature 008)."

## Clarifications

### Session 2026-11-05

- Q: KServe LLMInferenceService or the raw llm-d Helm charts (ADR-0002)? → A: KServe `LLMInferenceService` (`serving.kserve.io/v1alpha2`, KServe v0.21.0) as the per-model resource, installed without Knative, Istio or the predictive controller; this feature uses only its workload part (no `router`). The raw llm-d path (llm-d-modelservice chart + the Inference Extension chart) is documented in ADR-0002 as the fallback. (decided by agent: product spec FR-GW-4; the same resource later carries the scheduler/InferencePool for 011 and HPA/KEDA + WVA scaling for 012; the v0.21.0 controller only watches Gateway API, InferencePool, KEDA and LeaderWorkerSet types when their CRDs exist, so it runs before the gateway feature)
- Q: How is KServe installed? → A: Argo CD component `cert-manager` (jetstack chart v1.21.3, wave −2) and component `kserve` (OCI charts `kserve-llmisvc-crd` and `kserve-llmisvc-resources` v0.21.0, namespace `kserve`, wave −1) in every profile; model-server TLS stays off (chart default), so servers speak HTTP on port 8000. (decided by agent: KServe's webhooks need cert-manager; TLS between gateway and servers is revisited with the gateway in 009)
- Q: How are serving manifests produced from the catalog? → A: `aibox render serving --profile <p>` writes `platform/serving/overlays/<p>/models.yaml` (one LLMInferenceService per enabled, served model); the files are committed and a Go test fails on drift — the same pattern as the node pools. They contain no account data: the weights bucket reaches the pods at runtime from the ConfigMap `aibox-run` (key `weightsBucket`), which the bootstrap chart renders from the `weightsBucket` parameter that `aibox up` sets from the Terraform output. (decided by agent: reviewable diffs, one code path for every profile, no account IDs in git)
- Q: How do AIBox's runtime settings combine with KServe's default template? → A: The LLMInferenceService overrides the `main` container of KServe's `kserve-config-llm-template` (image, command, arguments, probes, resources) and keeps its volumes, security context and `preStop` delay; `storageInitializer.enabled: false` because AIBox loads weights itself (verified in the v0.21.0 controller: with the initializer disabled nothing is attached for the model URI). (decided by agent: smallest override that keeps KServe's defaults)
- Q: Which images and versions? → A: vLLM `vllm/vllm-openai:v0.33.0`, SGLang `lmsysorg/sglang:v0.5.23`, the simulator `ghcr.io/llm-d/llm-d-inference-sim:v1.1.0`, all pinned by digest; flags verified in the sources of these versions (vLLM `vllm/engine/arg_utils.py`; SGLang `sglang/srt/arg_groups`). (decided by agent: latest stable releases)
- Q: How does each loading method reach the runtime? → A: `s3-stream`: model path `s3://$(AIBOX_WEIGHTS_BUCKET)/models/<id>/<revision>/` with `--load-format runai_streamer` (both runtimes support it at the pinned versions) and the `model-server` service account (Pod Identity, read-only bucket access from 004). `nvme-cache`: an init container fetches the mirror into an `emptyDir` on the node's instance-store-backed ephemeral storage (node classes from 006) with `mirror.py fetch`, which checks every checksum, and the runtime loads the local copy. `hf-download`: the runtime downloads the pinned revision itself (optional token from an `hf-token` ExternalSecret in the serving namespace). `none`: simulator. (decided by agent: E7 compares the three without other changes)
- Q: How is "ready only after a warm-up request" implemented? → A: For vLLM and SGLang a `postStart` hook (Python, present in both images) waits for `/health` and sends one 1-token completion; Kubernetes runs no probes until the hook finishes, so readiness follows the warm-up. A startup probe covers the load budget (`startupTimeoutSeconds`, default 1800 s), liveness starts after it. The simulator has no weights to load and gets no hook. (decided by agent: no sidecar, no runtime patch)
- Q: How is draining configured? → A: Optional catalog field `shutdownGracePeriodSeconds` (default 120) sets `terminationGracePeriodSeconds`; KServe's `preStop` delay lets endpoints drop the pod, then vLLM drains with `--shutdown-timeout` (grace − 20 s); SGLang finishes within the grace period. (decided by agent: vLLM's documented drain flag; one knob per model)
- Q: Resources and placement? → A: `nvidia.com/gpu` = tensor parallel size; CPU 4 and memory 16 GiB per GPU (requests = limits for memory); `/dev/shm` 8 GiB per GPU for TP > 1; node selector `aibox.dev/gpu-class` = the model's `minGPUClass` and a toleration for `nvidia.com/gpu`. Rendering fails when the TP size exceeds the GPUs of the class's largest instance type in the profile. (decided by agent: simple, documented sizing; E-series experiments tune it)
- Q: How are quantization and the KV-cache type passed? → A: `quantization: fp8` → `--quantization fp8`, `awq` → `--quantization awq_marlin` (both runtimes), `bf16`/`fp16` → `--dtype`; `kvCacheDtype: fp8` → vLLM `fp8`, SGLang `fp8_e4m3`. (decided by agent: flag values verified in both sources)
- Q: Where do the metric names live and how is the contract tested? → A: `metrics/runtime-metrics.yaml` maps canonical names (queue, running, KV-cache usage, TTFT, ITL, end-to-end latency, prompt and generated tokens, request success) to the vLLM and SGLang names verified in their sources; `test/contract` checks recorded samples of both runtimes and, when `AIBOX_METRICS_URL` is set, a live endpoint (the simulator in e2e-local). (decided by agent: plan input; one file consumed by observability and autoscaling)
- Q: What serves the local endpoint until the gateway exists? → A: A NodePort Service `inference` (30080) in the local serving overlay selects the KServe-managed pods of the local model; feature 009 replaces it with the gateway. (decided by agent: keeps `aibox smoke` and the README unchanged)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Every catalog model is served from one definition (Priority: P1)

For each enabled, served catalog model of a profile, the platform runs a model server built
from the catalog entry: the runtime image pinned by digest, the runtime flags (prefix caching,
chunked prefill, GPU memory utilization, maximum concurrent sequences, KV-cache data type),
resources sized from the tensor parallel size, placement on the model's GPU class and the
weights loading method.

**Why this priority**: the runtime is where tokens are produced; nothing else works without it.

**Independent Test**: rendering the serving manifests for every profile from the catalog
matches the committed files (no drift), and the local profile serves its catalog model through
the same templates in CI.

**Acceptance Scenarios**:

1. **Given** the catalog and the `aws-bench` profile, **When** the serving manifests are
   rendered, **Then** each enabled model has one model server with the catalog's image,
   flags, tensor parallelism, GPU count and GPU class placement.
2. **Given** the `local` profile, **When** the platform comes up, **Then** the local catalog
   model answers chat completions on the local inference endpoint.

---

### User Story 2 - Ready only when it can answer (Priority: P1)

A model server reports ready only after its weights are loaded and one warm-up request
succeeded; long weight loads never trigger restarts.

**Why this priority**: routing traffic to a server that is still loading produces errors and
skews benchmark latency.

**Independent Test**: the rendered servers have a startup check with a budget of at least the
expected load time, a liveness check that only starts after startup, and a warm-up step
before readiness.

**Acceptance Scenarios**:

1. **Given** a model that takes 10 minutes to load, **When** it starts, **Then** it is not
   restarted and becomes ready only after the warm-up request succeeds.

---

### User Story 3 - Drain before stopping (Priority: P1)

When a model server is stopped (scale-in, node reclaim), it stops taking new requests and
finishes in-flight ones within a configurable grace period.

**Why this priority**: scale-in during benchmarks must not fail requests.

**Independent Test**: rendered servers carry the configured grace period and a drain step.

**Acceptance Scenarios**:

1. **Given** a grace period of 120 s in the catalog, **When** a server is stopped, **Then**
   it drains for up to 120 s before exiting.

---

### User Story 4 - The same metrics from every runtime (Priority: P1)

Every runtime exposes `/metrics` and `/health`. A contract test checks that the metrics AIBox
depends on (queue length, running requests, KV-cache usage, time to first token, inter-token
latency, end-to-end latency, prompt and generated tokens) exist under their runtime names,
and a mapping file translates runtime names to AIBox's canonical names for the observability
and autoscaling features.

**Why this priority**: autoscaling, routing and the report read these metrics; a silent rename
breaks experiments.

**Independent Test**: the contract test passes against the local simulator in CI and against
recorded metric samples of the pinned vLLM and SGLang versions.

**Acceptance Scenarios**:

1. **Given** the local platform, **When** the contract test scrapes the model server, **Then**
   every required vLLM metric is present.
2. **Given** a metric missing from a runtime, **When** the contract test runs, **Then** it
   fails naming the canonical metric and the runtime.

---

### User Story 5 - SGLang as an alternative runtime (Priority: P2)

`runtime: sglang` in the catalog serves the model with SGLang behind the same API, port,
probes and metric mapping.

**Why this priority**: experiment E4 compares runtimes; it needs no other change than the
catalog field.

**Independent Test**: the rendered server for an SGLang model uses the SGLang image and flags
and keeps the same port, probes and labels.

**Acceptance Scenarios**:

1. **Given** a model switched to `runtime: sglang`, **When** manifests are rendered, **Then**
   only the image, command and flags change.

---

### User Story 6 - FP8 weights and KV cache (Priority: P2)

FP8 or AWQ weights (`quantization`) and an FP8 KV cache (`kvCacheDtype`) are catalog options
passed to the runtime.

**Why this priority**: experiment E6 measures their effect.

**Independent Test**: rendering a model with `quantization: fp8` and `kvCacheDtype: fp8`
adds the matching runtime flags.

**Acceptance Scenarios**:

1. **Given** `kvCacheDtype: fp8`, **When** rendered for vLLM, **Then** the server starts with
   an FP8 KV cache.

### Edge Cases

- Out of memory while loading → the server keeps failing its startup check and is restarted;
  the runbook `docs/runbooks/model-load-failures.md` explains the causes and fixes.
- Tensor parallel size larger than the GPUs of the class's instance types → rendering fails
  with the model, the class and the GPU count.
- A runtime renames a metric in a new version → the contract test fails before the version
  bump merges.
- Weights not mirrored yet (S3 loading) → the server fails to start; the runbook points to
  `aibox models sync`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Each enabled, served catalog model of a profile MUST be rendered into exactly one
  per-model serving resource (the high-level resource chosen in ADR-0002) with the runtime
  image pinned by digest and the catalog's runtime flags.
- **FR-002**: Rendered servers MUST request the model's tensor parallel size in GPUs and be
  placed on the model's GPU class (selector and toleration); the local profile uses the fake
  GPU node.
- **FR-003**: Servers MUST expose `/health` and `/metrics`; readiness MUST follow weight loading
  and a successful warm-up request; a startup check MUST cover long loads.
- **FR-004**: Servers MUST drain in-flight requests on shutdown within a configurable grace
  period.
- **FR-005**: `runtime: sglang` MUST produce an SGLang server with the same API surface, port,
  probes and labels.
- **FR-006**: FP8/AWQ weights and FP8 KV cache MUST be expressible in the catalog and passed to
  the runtime.
- **FR-007**: A mapping file MUST translate each runtime's metric names to canonical names, and a
  contract test MUST verify the required metrics against the local simulator and the pinned
  runtime versions.
- **FR-008**: The same rendering MUST serve the local profile in CI.

### Key Entities

- **Model server**: one per served model and profile; runtime, image, flags, resources,
  placement, probes, grace period.
- **Runtime metric mapping**: canonical name → vLLM name, SGLang name, type.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: e2e-local deploys the local catalog model through the same templates used for
  AWS and the smoke test passes.
- **SC-002**: The metric contract test passes against the simulator in CI and against the
  pinned vLLM and SGLang metric names.
- **SC-003**: Rendered serving manifests for every profile are identical to the committed ones
  (no drift) and validate against the resource schemas.

## Assumptions

- Routing, gateway and autoscaling are out of scope (features 009, 011, 012); this feature
  only runs model servers and a local endpoint for the smoke test.
- Real GPU runs are not part of CI (agent hard stop); vLLM and SGLang flags are verified
  against their sources at the pinned versions.
