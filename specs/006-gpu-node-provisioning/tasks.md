# Tasks: Just-in-time GPU nodes

- [X] T001 Tests first: `internal/gpu/render_test.go` (pools per profile, spot/on-demand/reserved, limits, drift vs committed files)
- [X] T002 `internal/gpu/{instances.go,render.go}`; `aibox render nodepools`; `make generate`
- [X] T003 [P] `platform/karpenter/overlays/aws-*` (chart 1.15.1 via helmCharts, settings, digest)
- [X] T004 [P] `platform/gpu/base` (DCGM exporter) + overlays with generated `nodepools.yaml`; bootstrap values (waves)
- [X] T005 [P] kind worker2 fake GPU; simulator toleration/nodeSelector
- [X] T006 Scheduling test (krusty): serving tolerations/selectors match pools for every profile
- [X] T007 CRD schemas for Karpenter in `hack/schemas` (gen script) and kubeconform
- [X] T008 `hack/check-image-digests.sh` + CI step; pin DCGM digest from its output
- [X] T009 [P] Runbooks `gpu-nodes.md`, `spot-interruption.md`; ADR-0001 note
- [X] T010 DoD (`make validate-manifests`, e2e node label); PROGRESS

## Analysis log

`/speckit-analyze` pass 1: 0 CRITICAL/HIGH; 1 MEDIUM (SC-004 runbook mapped to T009). Coverage FR 8/8, SC 4/4.

## Convergence log

`/speckit-converge` pass 1 (2026-11-05): 2 findings, fixed in this pass.

- F1 `missing` (HIGH, FR-008/SC-001): CI rendered the Karpenter chart's
  CustomResourceDefinitions, for which kubeconform's registry has no schema → the schema of
  the CRD kind is generated from the Kubernetes 1.37.0 OpenAPI document
  (`hack/crd2schema -openapi`, `hack/gen-crd-schemas.sh`).
- F2 `missing` (HIGH, constitution II): the DCGM exporter image was not pinned → digest
  pinned from the `make check-image-digests` output in CI.

Pass 2: 0 findings. Evidence: `go test ./internal/gpu/... ./test/scheduling/...`,
`make validate-manifests` in CI (63/63 valid, Karpenter CRDs included), image digest check
green, e2e-local green (simulator scheduled on the fake GPU worker).
