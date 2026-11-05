# Tasks: Just-in-time GPU nodes

- [ ] T001 Tests first: `internal/gpu/render_test.go` (pools per profile, spot/on-demand/reserved, limits, drift vs committed files)
- [ ] T002 `internal/gpu/{instances.go,render.go}`; `aibox render nodepools`; `make generate`
- [ ] T003 [P] `platform/karpenter/overlays/aws-*` (chart 1.15.1 via helmCharts, settings, digest)
- [ ] T004 [P] `platform/gpu/base` (DCGM exporter) + overlays with generated `nodepools.yaml`; bootstrap values (waves)
- [ ] T005 [P] kind worker2 fake GPU; simulator toleration/nodeSelector
- [ ] T006 Scheduling test (krusty): serving tolerations/selectors match pools for every profile
- [ ] T007 CRD schemas for Karpenter in `hack/schemas` (gen script) and kubeconform
- [ ] T008 `hack/check-image-digests.sh` + CI step; pin DCGM digest from its output
- [ ] T009 [P] Runbooks `gpu-nodes.md`, `spot-interruption.md`; ADR-0001 note
- [ ] T010 DoD (`make validate-manifests`, e2e node label); PROGRESS

## Analysis log

`/speckit-analyze` pass 1: 0 CRITICAL/HIGH; 1 MEDIUM (SC-004 runbook mapped to T009). Coverage FR 8/8, SC 4/4.
