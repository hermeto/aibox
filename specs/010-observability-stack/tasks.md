# Tasks: Observability stack

- [x] T001 Tests first: rule generation golden/drift tests (canonical series, cost, SLO alerts)
- [x] T002 `internal/observability/rules.go`; `aibox render rules`; `make generate`
- [x] T003 [P] `platform/observability` (kube-prometheus-stack, Tempo, Loki, Collector, data sources, local NodePort 30300, monitors)
- [x] T004 [P] Static alerts + `promtool` unit tests; CI step
- [x] T005 [P] Five dashboards (SLO, capacity, GPU, cost, scaling) + dashboard lint in CI
- [x] T006 Gateway tracing to the Collector; PodMonitor for model servers; DCGM/Karpenter monitors
- [x] T007 e2e-local: Prometheus has simulator series, Tempo has a gateway trace, Grafana dashboards load
- [x] T008 [P] ADR-0011, runbook `observability.md`, README
- [x] T009 DoD; PROGRESS

## Analysis log

`/speckit-analyze` pass 1: 0 CRITICAL/HIGH. Coverage FR 6/6 (FR-001→T003/T006, FR-002→T003/T005,
FR-003→T002/T004, FR-004→T002, FR-005→T004/T005, FR-006→T003), SC 3/3 (SC-001→T007,
SC-002→T004, SC-003→T005).

## Verification log

- `go test ./internal/observability/...` — generation and dashboard tests pass.
- `promtool test rules platform/observability/rules/tests/alerts.test.yaml` — SUCCESS.
- `kustomize build --enable-helm platform/observability/overlays/{local,aws-dev,aws-bench}` —
  renders; every image in the local overlay is pinned by digest.
- `make generate` drift — generated rules unchanged.
- e2e assertions run in CI (`TestLocalPlatform` → `checkObservability`); Docker was not
  available in the implementation session.
