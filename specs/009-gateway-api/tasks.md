# Tasks: OpenAI-compatible gateway

- [ ] T001 Profile `spec.gateway.rateLimits` (schema, types, defaults, tests); catalog `routing`
- [ ] T002 Tests first: gateway rendering golden/drift tests (routes, backends, policies, fallback 404)
- [ ] T003 `internal/serving/gateway.go`; render into `platform/serving/overlays/*/models.yaml`
- [ ] T004 [P] `platform/gateway` (Envoy Gateway + AI Gateway charts, Redis, GatewayClass, EnvoyProxy, Gateway, ClientTrafficPolicy); overlays local (NodePort 30080) and AWS (NLB); bootstrap values; CRD schemas
- [ ] T005 CLI: API key Secret at bring-up (`.aibox/<profile>/api-key`), `--tls-cert-arn`, admin source ranges; smoke through the gateway; metrics NodePort 30082 + kind mapping
- [ ] T006 e2e-local gateway tests: 401, 429 (requests, tokens), 404, routing, streaming
- [ ] T007 [P] ADR-0003, runbook `gateway.md`, README
- [ ] T008 DoD (`make e2e-local`, smoke JSON has no failing checks); PROGRESS

## Analysis log

`/speckit-analyze` pass 1: 0 CRITICAL/HIGH. Coverage FR 8/8 (FR-001→T003/T004, FR-002→T003,
FR-003→T004/T005, FR-004→T001/T003, FR-005→T003, FR-006→T004/T005, FR-007→T006,
FR-008→T005), SC 3/3 (SC-001→T006, SC-002→T005, SC-003→T002/T004).
