# Tasks: Model catalog

- [X] T001 `schemas/catalog.schema.json`, `models/catalog.yaml`, `internal/models/catalog.go` + tests (fixtures)
- [X] T002 `aibox models list` (text/JSON, profile filter)
- [X] T003 Tests first: Job rendering golden tests (mirror, load-check) and sync orchestration with a fake kube API
- [X] T004 `internal/models/{jobs.go,sync.go}`, kube job helpers; `aibox models sync`
- [X] T005 [P] `platform/models` (SeaweedFS local) and `platform/secrets` (ESO, stores, hf-token); bootstrap values
- [X] T006 `hack/hfcheck` + CI step; pin revisions from its output
- [X] T007 e2e: sync the tiny model in `TestLocalPlatform`
- [X] T008 [P] ADR-0007 (initial); docs
- [X] T009 DoD (`go test ./internal/models/...`, `models list -o json`, e2e); PROGRESS

## Analysis log

`/speckit-analyze` pass 1: 0 CRITICAL/HIGH. Coverage FR 6/6, SC 3/3 (SC-003 → T003 orchestration test).

## Convergence log

`/speckit-converge` pass 1 (2026-11-05): 2 findings, fixed in this pass.

- F1 `contradicts` (HIGH, FR-004): the ESO webhook rejects fake-provider entries with empty
  values, so the local ClusterSecretStore was never created and the `models` application
  degraded → the local store holds no token; the ExternalSecret (`deletionPolicy: Delete`)
  then resolves to no Secret, the same path as a cloud account without a stored token.
- F2 `missing` (HIGH, FR-006): catalog revisions were placeholders → pinned the commits that
  `make check-model-revisions` printed in CI.

Pass 2: 0 findings. Evidence: `go test ./internal/models/... ./hack/hfcheck/...` (91% / 66%),
CI `lint` incl. `make check-model-revisions` (5/5 revisions resolvable) and
`make validate-manifests`, `e2e-local`: bring-up 3m9s, smoke green, sync 1 "synced: 10 files,
10 uploaded (14882707 bytes); verified: 10 files", sync 2 "skipped: already mirrored; verified:
10 files".
