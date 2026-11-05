# Tasks: Inference runtimes for catalog models

- [ ] T001 Catalog: `shutdownGracePeriodSeconds`, `startupTimeoutSeconds` (schema, types, tests)
- [ ] T002 Tests first: `internal/serving` golden tests (vLLM s3-stream TP=2, SGLang, nvme-cache, hf-download, simulator), validation tests (TP > GPUs, unsupported combinations), drift test vs `platform/serving/overlays/*/models.yaml`
- [ ] T003 `internal/serving/{render.go,runtimes.go,warmup.py}`; `aibox render serving`; `make generate`
- [ ] T004 `mirror.py fetch` (nvme-cache) + tests of the rendered init container
- [ ] T005 [P] `platform/cert-manager/base`, `platform/kserve/base` (charts pinned, digests), bootstrap values (waves) for every profile
- [ ] T006 [P] Bootstrap `aibox-run` ConfigMap (`weightsBucket`, `runId`, `region`); `aibox up` passes `weightsBucket` for cloud profiles
- [ ] T007 Local serving overlay: generated LLMInferenceService for the simulator + NodePort `inference`; remove the hand-written Deployment; scheduling test update
- [ ] T008 `metrics/runtime-metrics.yaml` + `test/contract` (fixtures for vLLM 0.33.0 and SGLang 0.5.23, live mode); e2e-local runs the live contract test
- [ ] T009 CRD schema for LLMInferenceService (gen script), `make validate-manifests`, image digests (cert-manager from CI)
- [ ] T010 [P] ADR-0002 (Accepted), runbook `model-load-failures.md`, README/docs
- [ ] T011 DoD (`make e2e-local` in CI, `go test ./test/contract/... -run TestRuntimeMetrics`); PROGRESS

## Analysis log

`/speckit-analyze` pass 1: 0 CRITICAL/HIGH; 1 MEDIUM resolved in place (FR-004 drain for
SGLang has no runtime flag — the clarification states the grace period applies; no task
change). Coverage FR 8/8 (FR-001→T002/T003, FR-002→T003/T007, FR-003→T003, FR-004→T003,
FR-005→T002/T003, FR-006→T001/T003, FR-007→T008, FR-008→T007), SC 3/3 (SC-001→T007/T011,
SC-002→T008, SC-003→T002/T009).
