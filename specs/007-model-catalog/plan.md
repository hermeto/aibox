# Implementation Plan: Model catalog and model weight lifecycle

**Branch**: `007-model-catalog` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

## Summary

`models/catalog.yaml` + `schemas/catalog.schema.json` + `internal/models` (typed loader,
filters, validation). `aibox models list|sync`. Sync renders two `batch/v1` Jobs per model
(mirror, load-check) in `aibox-models`, applies them with server-side apply and waits through
the Kubernetes API. Local: SeaweedFS 4.52 in `platform/models` (local overlay) and ESO
2.14.0 with a fake store in `platform/secrets`; AWS: ESO with AWS Secrets Manager.
`hack/hfcheck` verifies/pins revisions in CI. e2e-local runs the tiny model sync.

## Technical Context

**Language/Version**: Go 1.27.3.

**Primary Dependencies**: SeaweedFS `chrislusf/seaweedfs:4.52@sha256:57877e32…`,
`peakcom/s5cmd:v2.3.0@sha256:2ff939e2…`, `python:3.15-slim@sha256:8c13a3f3…` +
`huggingface_hub==2.4.0`, `busybox:1.37@sha256:a7dbc319…`, External Secrets chart 2.14.0.

**Testing**: schema/fixture tests; Job rendering golden tests; sync orchestration with a fake
Kubernetes API; e2e sync in CI.

**Constraints**: no Hugging Face access from the build environment (CI resolves revisions).

## Constitution Check

| Principle | Status | Evidence |
| --- | --- | --- |
| I | Pass | ADR-0007 (initial: loading methods, decided by E7). |
| II | Pass | Revisions are commit SHAs (CI-verified); images by digest; pip pin. |
| III | Pass | SeaweedFS + fake secret store locally; e2e sync. |
| IV | Pass | Weights bucket in Terraform; nothing extra billed. |
| V | Pass | Store and ESO delivered by Argo CD; sync is an explicit operation. |
| VI | Pass | Schema, render and orchestration tests; e2e. |
| VII | Pass | Job status surfaced by the CLI; Job logs. |
| VIII | Pass | Token only via ExternalSecret; restricted pod security on Jobs. |
| IX | Pass | No custom images. |
| X | Pass | Docs updated. |

## Project Structure

```text
models/catalog.yaml, schemas/catalog.schema.json
internal/models/{catalog.go,jobs.go,sync.go} (+ tests, testdata)
internal/cli/models.go
internal/kube/kube.go            # job status helper
platform/models/{base,overlays/local}   # SeaweedFS (local only)
platform/secrets/{base,overlays/local,overlays/aws-dev,overlays/aws-bench}  # ESO + stores + hf-token
hack/hfcheck/main.go
docs/adr/0007-weight-loading-and-cold-start.md
```

## Complexity Tracking

No violations.
