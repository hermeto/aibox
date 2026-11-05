# ADR-0007: Weight loading and cold start

- **Status**: Proposed — pending experiment E7
- **Date**: 2026-11-05
- **Deciders**: AIBox maintainers (initial version written by the agent in feature 007)
- **Decided by**: experiment E7

## Context and problem statement

Scaling out an LLM server is dominated by getting tens of gigabytes of weights onto the GPU.
AIBox must make the weight source reproducible (pinned revisions, checksums, no runtime
dependency on Hugging Face availability) and must let experiment E7 compare loading methods
without code changes (product spec FR-MOD-3, E7). Feature 007 delivers the mirror; this ADR
records the initial default and how it will be decided.

## Decision drivers

- Reproducible: a run serves exactly the pinned commit, verified by checksums.
- Fast cold start: seconds rather than minutes when Karpenter adds a GPU node.
- Works without GPUs: the mirror path is tested on kind in CI.
- No custom images, nothing extra billed when the cluster is gone.

## Considered options

1. **Hugging Face download at start** (`hf-download`): simplest; every replica downloads from
   the internet; slow and dependent on Hugging Face rate limits and availability.
2. **S3 streaming** (`s3-stream`): weights mirrored once per run to the run's bucket, then
   streamed straight to GPU memory (vLLM `--load-format runai_streamer`, verified against the
   pinned vLLM version in feature 008).
3. **NVMe cache** (`nvme-cache`): an init container copies the mirror to the node's instance
   store (Karpenter node classes format it, feature 006), and the server loads from local disk;
   later replicas on the same node start from the cache.

## Decision outcome

Initial default: **S3 streaming** for every catalog model (`loadMethod: s3-stream`), with the
other methods selectable per model in `models/catalog.yaml`. The mirror itself is decided:
`aibox models sync` copies each pinned revision to `s3://<bucket>/models/<id>/<revision>/` with
`SHA256SUMS` and a `.complete` marker written last, file by file (download, check the SHA-256
that Hugging Face publishes, upload), and a verify Job streams the objects back and checks
every checksum. E7 measures time-to-first-token after scale-out for the three methods on the
same model and node type and updates this ADR to Accepted with the measured default.

### Consequences

- Good: weights come from the run's own bucket (no Hugging Face dependency during
  benchmarks); E7 needs only a catalog change per variant.
- Good: the mirror is idempotent and resumable, and runs on kind against a local S3 store.
- Bad: one extra copy of the weights per run (S3 storage, deleted with the run's bucket).
- Bad: streaming depends on the runtime's loader support; runtimes without it fall back to
  `nvme-cache` (feature 008 records which runtime supports which method).

## Links

- Spec: `specs/007-model-catalog/spec.md` (FR-003, User Story 3)
- Product spec: §A.10 (ADR index), E7 (cold start)
