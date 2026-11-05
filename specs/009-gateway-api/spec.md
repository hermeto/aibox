# Feature Specification: OpenAI-compatible gateway with authentication, rate limits and baseline routing

**Feature Branch**: `009-gateway-api`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "Single entry point for inference traffic. Clients need one stable, OpenAI-compatible endpoint with authentication and rate limits; the benchmark needs a naive round-robin routing mode as the baseline for experiment E1. Gateway API implementation decided in ADR-0003; API keys synced from Secrets Manager on AWS; per-key requests/min and tokens/min limits; routing by the body's `model` field; round-robin mode selectable per model without redeploying model servers (EPP mode added in 011); TLS on AWS via the load balancer and ACM; plain HTTP on localhost locally; streaming without buffering (playbook §6, feature 009)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One OpenAI-compatible endpoint (Priority: P1)

A client sends `POST /v1/chat/completions` and `POST /v1/completions`, and lists models with
`GET /v1/models`, on one hostname; the `model` field of the body selects the model server.

**Why this priority**: every client, the smoke test and the benchmark use this endpoint.

**Independent Test**: in CI on kind, requests through the gateway reach the local model; an
unknown model gets an OpenAI-style 404.

**Acceptance Scenarios**:

1. **Given** the local platform, **When** a client posts a chat completion for `sim-llama-8b`
   with a valid key, **Then** it gets 200 and the completion.
2. **Given** a request for a model that does not exist, **When** it reaches the gateway,
   **Then** the response is 404 with an OpenAI-style error body.
3. **Given** a client, **When** it calls `GET /v1/models`, **Then** the served models are listed.

---

### User Story 2 - API keys (Priority: P1)

Requests must carry a valid API key; keys live in Kubernetes Secrets synced from the secret
store (AWS Secrets Manager on cloud profiles), never in git.

**Why this priority**: an open endpoint on the internet costs money and leaks capacity.

**Independent Test**: requests without a key or with a wrong key get 401 in CI.

**Acceptance Scenarios**:

1. **Given** no `Authorization` header, **When** a request arrives, **Then** it gets 401.
2. **Given** a key that is not configured, **When** a request arrives, **Then** it gets 401.

---

### User Story 3 - Per-key rate limits (Priority: P1)

Each key has a limit in requests per minute and in tokens per minute; requests beyond the
limit get 429.

**Why this priority**: protects shared capacity during benchmark campaigns.

**Independent Test**: in CI, a key with a low limit receives 429 after the limit.

**Acceptance Scenarios**:

1. **Given** a key limited to N requests per minute, **When** it sends N+1 requests within a
   minute, **Then** the last one gets 429.
2. **Given** a key limited in tokens per minute, **When** its requests use more tokens than the
   limit, **Then** further requests get 429 until the window resets.

---

### User Story 4 - Baseline round-robin routing (Priority: P1)

Per model, the routing mode `round-robin` sends requests across the model's servers through
a plain Service; it is selectable without redeploying the model servers (the Endpoint Picker
mode arrives in feature 011).

**Why this priority**: experiment E1 compares smart routing against this baseline.

**Independent Test**: the rendered route for a model in `round-robin` mode targets the model's
Service; switching the mode changes only routing objects.

**Acceptance Scenarios**:

1. **Given** a model in `round-robin` mode with two servers, **When** requests arrive, **Then**
   both servers receive traffic.

---

### User Story 5 - TLS on AWS, HTTP on localhost (Priority: P2)

Cloud profiles terminate TLS at the cloud load balancer with a certificate from ACM; the local
profile serves plain HTTP on 127.0.0.1.

**Why this priority**: credentials must not cross the internet in clear text.

**Independent Test**: the AWS rendering requests a TLS listener with the configured
certificate; the local rendering exposes port 30080 on 127.0.0.1.

---

### User Story 6 - Streaming (Priority: P2)

Server-sent-event responses stream through the gateway without buffering; a client
disconnect cancels the upstream request.

**Independent Test**: in CI, a streamed completion arrives as several chunks through the
gateway.

### Edge Cases

- Very large prompts → the gateway rejects bodies above a configured size with 413.
- Client disconnects mid-stream → the upstream request is cancelled.
- Model server not ready → 503 with an OpenAI-style error body.
- Rate-limit store unavailable → requests are allowed (fail open) and the condition is
  observable (documented).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The platform MUST expose one OpenAI-compatible entry (`/v1/chat/completions`,
  `/v1/completions`, `/v1/models`) through the Gateway API implementation chosen in ADR-0003.
- **FR-002**: Requests MUST be routed by the body's `model` field to that model's servers;
  unknown models MUST get 404 with an OpenAI-style error body.
- **FR-003**: Requests MUST carry a valid API key; keys MUST come from Kubernetes Secrets
  synced from the profile's secret store.
- **FR-004**: Each key MUST have request-per-minute and token-per-minute limits; excess
  requests MUST get 429.
- **FR-005**: A `round-robin` routing mode per model MUST target a plain Service and be
  switchable without redeploying model servers.
- **FR-006**: Cloud profiles MUST terminate TLS at the load balancer (ACM certificate); the
  local profile MUST serve HTTP on 127.0.0.1.
- **FR-007**: Streaming responses MUST pass through unbuffered.
- **FR-008**: `aibox smoke` MUST use the gateway (with a key) for its checks.

### Key Entities

- **API key**: id, secret value, request and token limits.
- **Model route**: model name, routing mode (`round-robin` | `epp` in 011), backend.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: e2e-local covers auth (401), rate limiting (429), model routing (200/404) and
  streaming through the gateway.
- **SC-002**: `aibox smoke --profile local` passes through the gateway.
- **SC-003**: Rendered routing objects for every profile validate against their schemas.

## Assumptions

- Smart routing (InferencePool + Endpoint Picker scoring) is feature 011; multi-tenant
  quotas and billing are out of scope.
- Tokens per minute are counted from the token usage the runtime reports in responses.
