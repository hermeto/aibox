# ADR-0003: Envoy Gateway with the Envoy AI Gateway as the inference gateway

- **Status**: Accepted
- **Date**: 2026-11-05
- **Deciders**: AIBox maintainers (proposed by the agent in feature 009)
- **Decided by**: feature 009

## Context and problem statement

AIBox needs one OpenAI-compatible entry point (`/v1/chat/completions`, `/v1/completions`,
`/v1/models`) with API keys, per-key limits in requests and tokens per minute, routing by the
body's `model` field, a naive round-robin baseline (experiment E1) and, in feature 011,
InferencePool backends with the llm-d Endpoint Picker (product spec FR-GW-1…6).

## Decision drivers

- Gateway API Inference Extension support (InferencePool backends) for feature 011.
- Model routing from the request body without custom code.
- API keys and token-based rate limits without an external authorization service.
- Same gateway on kind and EKS; multi-arch images; active community.

## Considered options

1. **Envoy Gateway + Envoy AI Gateway** (v1.9.3 / v1.2.0).
2. **kgateway** with the Inference Extension body-based router.
3. **Istio** with the Inference Extension.

## Decision outcome

Chosen option: "Envoy Gateway + Envoy AI Gateway", because the AI Gateway extracts the model
from the body into a routing header, answers `/v1/models` from the routes, records token usage
for cost-based rate limits (`llmRequestCosts`) and routes to `InferencePool` backends, while
Envoy Gateway provides `SecurityPolicy.apiKeyAuth` (Envoy strips the "Bearer " prefix, so OpenAI clients
work unchanged) and global rate limits keyed on the forwarded client id. KServe v0.21.0's own
installer pairs the same projects. kgateway and Istio would need an external authorization
service for keys and custom filters for token limits.

### Consequences

- Good: keys, request and token limits, model routing and `/v1/models` are configuration.
- Good: switching a model to the Endpoint Picker (011) changes one route rule's backend.
- Bad: three components (Envoy Gateway, AI Gateway controller, Redis for limits) and an
  extension-manager hook in Envoy Gateway's configuration.
- Bad: AI Gateway backends must be Envoy Gateway `Backend` objects (v1.2.0), so the
  round-robin baseline goes through a per-model headless Service.
- Rate limiting fails open when Redis is unavailable (Envoy's rate-limit service default).

## Links

- Spec: `specs/009-gateway-api/spec.md` (Clarifications) and `research.md`.
- Product spec: FR-GW-1, FR-GW-3, FR-GW-5, FR-GW-6.
