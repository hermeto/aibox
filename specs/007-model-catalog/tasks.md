# Tasks: Model catalog

- [ ] T001 `schemas/catalog.schema.json`, `models/catalog.yaml`, `internal/models/catalog.go` + tests (fixtures)
- [ ] T002 `aibox models list` (text/JSON, profile filter)
- [ ] T003 Tests first: Job rendering golden tests (mirror, load-check) and sync orchestration with a fake kube API
- [ ] T004 `internal/models/{jobs.go,sync.go}`, kube job helpers; `aibox models sync`
- [ ] T005 [P] `platform/models` (SeaweedFS local) and `platform/secrets` (ESO, stores, hf-token); bootstrap values
- [ ] T006 `hack/hfcheck` + CI step; pin revisions from its output
- [ ] T007 e2e: sync the tiny model in `TestLocalPlatform`
- [ ] T008 [P] ADR-0007 (initial); docs
- [ ] T009 DoD (`go test ./internal/models/...`, `models list -o json`, e2e); PROGRESS

## Analysis log

`/speckit-analyze` pass 1: 0 CRITICAL/HIGH. Coverage FR 6/6, SC 3/3 (SC-003 → T003 orchestration test).
