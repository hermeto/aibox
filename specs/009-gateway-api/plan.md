# Implementation Plan: OpenAI-compatible gateway

**Branch**: `009-gateway-api` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

## Summary

Component `gateway` installs Envoy Gateway v1.9.3 and Envoy AI Gateway v1.2.0 (charts pinned,
images by digest), a Redis for global rate limits, and the `aibox` GatewayClass, EnvoyProxy
and Gateway (NodePort 30080 locally, NLB on AWS). `internal/serving` additionally renders per
model a headless Service, an Envoy Gateway `Backend` and an `AIServiceBackend`, and one
`AIGatewayRoute` (rule per model, `round-robin` routing), a fallback HTTPRoute answering
OpenAI-style 404s, the `SecurityPolicy` (API keys) and the `BackendTrafficPolicy` (requests
and tokens per minute from the profile). `aibox up` creates the CLI key; `aibox smoke` goes
through the gateway. e2e-local tests 401, 429, 404, routing and streaming.

## Technical Context

**Language/Version**: Go 1.27.3.

**Primary Dependencies**: `oci://docker.io/envoyproxy/gateway-helm` v1.9.3,
`oci://docker.io/envoyproxy/ai-gateway-crds-helm` and `ai-gateway-helm` v1.2.0,
`redis:8` (digest), Gateway API v1.6.1 CRDs (bundled with the Envoy Gateway chart).

**Testing**: rendering golden/drift tests, profile schema tests, kubeconform with the CRD
schemas of Envoy Gateway and the AI Gateway, e2e-local gateway tests (Go `net/http`).

## Constitution Check

| Principle | Status | Evidence |
| --- | --- | --- |
| I | Pass | ADR-0003 (Accepted). |
| II | Pass | Charts, images and CRDs pinned. |
| III | Pass | Same gateway locally (NodePort) and on AWS (NLB). |
| IV | Pass | One NLB per run, destroyed with the cluster (orphan check covers it). |
| V | Pass | Delivered by Argo CD; keys created by the CLI or synced by ESO. |
| VI | Pass | Unit and e2e tests for auth, limits, routing, streaming. |
| VII | Pass | Gateway metrics scraped in 010; client id forwarded for attribution. |
| VIII | Pass | Keys never in git; TLS on AWS; source ranges restricted. |
| IX | Pass | Upstream charts and images only. |
| X | Pass | ADR-0003, runbook `gateway.md`. |

## Project Structure

```text
platform/gateway/{base,overlays/local,overlays/aws-dev,overlays/aws-bench}
internal/serving/gateway.go (+ tests)        # routes, backends, policies
schemas/profile.schema.json, internal/profile  # spec.gateway.rateLimits
internal/platform (API key Secret, TLS/source-range parameters), internal/cli (smoke, up flags)
test/e2e/gateway_test.go
docs/adr/0003-gateway-implementation.md, docs/runbooks/gateway.md
```

## Complexity Tracking

No violations.
