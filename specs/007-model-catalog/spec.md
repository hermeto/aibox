# Feature Specification: Model catalog and model weight lifecycle

**Feature Branch**: `007-model-catalog`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "models/catalog.yaml declares each model (source, pinned revision, license, quantization, tensor parallel size, max model length, minimum GPU class, runtime and flags, SLO targets) and is validated by a JSON Schema; `aibox models list`; `aibox models sync` mirrors weights from Hugging Face to the environment's object storage with an in-cluster Job (checksums, idempotent, resumable) using a token from a secret store; per-model loading method (S3 streaming or NVMe cache); the local profile uses an in-cluster S3-compatible store so the sync path is tested in CI (playbook §6, feature 007)."

## Clarifications

### Session 2026-11-05

- Q: Which models are the v1.0 defaults? → A: Apache-2.0 first: `qwen3-8b` (Qwen/Qwen3-8B, baseline 8B, TP=1, gpu-l4), `qwen3-32b-fp8` (Qwen/Qwen3-32B-FP8, TP=2, gpu-l4 on g6.12xlarge), `qwen2.5-72b-awq` (Qwen/Qwen2.5-72B-Instruct-AWQ, 70B-class quantized, TP=4, gpu-l4 on g6.12xlarge; Qwen license). `llama-3.1-8b-instruct` is in the catalog but `enabled: false` (Llama 3.1 Community License, gated: accept on Hugging Face first). Local: `sim-llama-8b` (simulator) and `tiny-random-llama` (hf-internal-testing/tiny-random-LlamaForCausalLM, a few MB) for the sync test. (decided by agent: product spec prefers permissive defaults; sizes chosen to fit the profiles' GPU classes)
- Q: How are revisions pinned when Hugging Face is unreachable from the build environment? → A: `revision` must be a 40-hex commit SHA; `hack/hfcheck` (CI) verifies every revision still exists and, for a placeholder of 40 zeros, fails printing the current `main` SHA to pin. (decided by agent: same pattern as image digests; also covers "revision no longer available")
- Q: Which S3-compatible store runs locally? → A: SeaweedFS 4.52 (single `weed server -s3`), because MinIO stopped publishing community container images; the bucket is `aibox-weights`. (decided by agent: maintained, signed images; S3 API sufficient for s5cmd)
- Q: Which image runs the sync? → A: No custom image: an init container (`python:3.15-slim` + `huggingface_hub==2.4.0` via pip at a pinned version) downloads and writes `SHA256SUMS`; a container with `peakcom/s5cmd:v2.3.0` uploads with `sync` (resumable) and writes a `.complete` marker last (idempotent: a present marker skips the work). A second Job (`busybox`) downloads and verifies `SHA256SUMS` — "a pod loads it". (decided by agent: plan input; smallest trusted images)
- Q: Where does the Hugging Face token come from? → A: External Secrets Operator 2.14.0: `ClusterSecretStore aibox` backed by AWS Secrets Manager (key `aibox/<cluster>/hf-token`, Pod Identity role from 004) on AWS, and by the ESO `fake` provider (empty token) locally; `ExternalSecret hf-token` in `aibox-models`. (decided by agent: product spec FR-MOD-4)
- Q: How is the loading method represented? → A: `loadMethod: s3-stream | nvme-cache | hf-download` per model (default `s3-stream`); runtimes (008) implement it. (decided by agent: E7 compares methods without catalog changes)
- Q: Object layout? → A: `s3://<bucket>/models/<id>/<revision>/…` plus `SHA256SUMS` and `.complete`. (decided by agent: immutable per revision; easy cleanup)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Declare models once (Priority: P1)

An operator declares each model in `models/catalog.yaml` — id, source repository, pinned
revision, license, quantization, tensor parallel size, max model length, minimum GPU class,
runtime and runtime flags, SLO targets (TTFT p99, ITL p99), loading method — and the file is
validated in CI. `aibox models list` prints it (text or JSON).

**Independent Test**: the shipped catalog validates; invalid fixtures fail with the field
path; `aibox models list --output json` lists every model.

**Acceptance Scenarios**:

1. **Given** the catalog, **When** `aibox models list --profile aws-bench` runs, **Then** the
   enabled models for that profile are listed with runtime, GPU class, TP and SLO.
2. **Given** an unknown field or a non-SHA revision, **When** validated, **Then** it fails with
   the path.

---

### User Story 2 - Mirror weights into the environment (Priority: P1)

`aibox models sync --profile <p>` runs in-cluster Jobs that mirror each model's weights from
Hugging Face to the environment's bucket, verifying checksums, skipping models already
mirrored and resuming partial uploads. The Hugging Face token comes from a secret store,
never from git.

**Independent Test**: in CI on kind, syncing the tiny model to the local store succeeds and a
load-check Job downloads it and verifies every checksum.

**Acceptance Scenarios**:

1. **Given** the local platform, **When** `aibox models sync --profile local --model
   tiny-random-llama` runs, **Then** the weights land under `models/<id>/<revision>/` with
   `SHA256SUMS` and `.complete`, and the load check passes.
2. **Given** a model already mirrored, **When** sync runs again, **Then** it is skipped.
3. **Given** a gated model without license acceptance or token, **When** sync runs, **Then**
   the Job fails with the Hugging Face error and the CLI explains how to accept the license.

---

### User Story 3 - Choose how weights are loaded (Priority: P1)

Each model selects its loading method — streaming from S3, a local NVMe cache, or a direct
Hugging Face download — so experiment E7 can compare them without code changes.

**Independent Test**: the catalog schema accepts the three methods; the runtime templates
(008) read the field.

**Acceptance Scenarios**:

1. **Given** `loadMethod: nvme-cache`, **When** runtimes are rendered (008), **Then** pods
   copy weights to the node's NVMe before starting.

### Edge Cases

- Gated model without acceptance → Job fails; CLI message names the license page.
- Revision no longer available → `hack/hfcheck` fails in CI; sync fails with 404.
- Partial upload → `s5cmd sync` uploads only missing/changed files; the marker is written last.
- Local store unavailable → sync fails fast with the endpoint in the message.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The catalog MUST declare per model: id, source, revision (commit SHA), license,
  quantization, tensor parallel size, max model length, minimum GPU class, runtime
  (vllm | sglang | simulator), runtime flags, SLO (TTFT p99, ITL p99), loading method,
  profiles and enabled flag; it MUST validate against `schemas/catalog.schema.json`.
- **FR-002**: `aibox models list` MUST print the catalog (text/JSON), filtered by profile.
- **FR-003**: `aibox models sync` MUST mirror weights with in-cluster Jobs, verify checksums,
  be idempotent and resumable, and report per-model status.
- **FR-004**: The Hugging Face token MUST come from a secret store via External Secrets.
- **FR-005**: The local profile MUST run an S3-compatible store so the sync path runs in CI.
- **FR-006**: Revisions MUST be verified to exist (CI) and pinned as commit SHAs.

### Key Entities

- **Model**: catalog entry (see FR-001).
- **Mirror**: `models/<id>/<revision>/` + `SHA256SUMS` + `.complete` in the bucket.

## Success Criteria *(mandatory)*

- **SC-001**: e2e-local syncs the tiny model to the local store and a pod verifies it.
- **SC-002**: 100% of catalog entries validate and have resolvable revisions (CI).
- **SC-003**: A second sync of the same model performs no upload (skipped).

## Assumptions

- Model licenses are respected (NFR-11): gated models are disabled by default.
- Quality evaluation and LoRA adapters are out of scope (v2).
