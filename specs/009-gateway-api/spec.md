# Feature Specification: OpenAI-compatible gateway with authentication, rate limits and baseline routing

**Feature Branch**: `009-gateway-api`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "Single entry point for inference traffic. Clients need one stable, OpenAI-compatible endpoint with authentication and rate limits; the benchmark needs a naive round-robin routing mode as the baseline for experiment E1. Gateway API implementation decided in ADR-0003; API keys synced from Secrets Manager on AWS; per-key requests/min and tokens/min limits; routing by the body's `model` field; round-robin mode selectable per model without redeploying model servers (EPP mode added in 011); TLS on AWS via the load balancer and ACM; plain HTTP on localhost locally; streaming without buffering (playbook §6, feature 009)."

## Clarifications

### Session 2026-11-05

- Q: Which Gateway API implementation (ADR-0003)? → A: Envoy Gateway v1.9.3 with Envoy AI Gateway v1.2.0. (decided by agent: the AI Gateway extracts the body's `model` into a routing header, answers `/v1/models`, counts token usage for rate limits and routes to InferencePools for 011; Envoy Gateway is a Gateway API Inference Extension implementation, has API-key auth and global rate limits, ships multi-arch images, and is what KServe's installer pairs with v0.21; kgateway and Istio need external auth or custom filters for keys and token limits)
- Q: How are the gateway and its dependencies installed? → A: Argo CD component `gateway` (wave −2, before KServe so its controller sees the Gateway API CRDs at start): the Envoy Gateway chart (with the AI Gateway extension-manager settings and a Redis rate-limit backend), the AI Gateway CRD and controller charts, a single Redis 8 instance for rate-limit counters, the `aibox` GatewayClass, EnvoyProxy and Gateway. Locally the Envoy Service is a NodePort on 30080 (the existing inference port); on AWS it is a Network Load Balancer. (decided by agent: one component owns the entry point)
- Q: How are models routed? → A: `aibox render serving` also renders, per served model, a headless Service of its pods, an Envoy Gateway `Backend` (FQDN of that Service, so Envoy balances per request across pod addresses) and an `AIServiceBackend`, plus one `AIGatewayRoute` with a rule per model matching the model header. The catalog field `routing: round-robin` (default) selects this backend; `epp` arrives in 011 and only swaps the rule's backend, so model servers are not redeployed. Unknown models fall through to an HTTPRoute that answers 404 with an OpenAI-style error body. (decided by agent: AI Gateway backends must be Envoy Gateway `Backend` objects in v1.2.0)
- Q: Where do API keys come from? → A: Envoy Gateway `SecurityPolicy` with `apiKeyAuth` on the `Authorization` header (Envoy strips `Bearer `), credentials from the Secret `aibox-api-keys` (data: client id → key). `aibox up` creates the Secret with a generated key for the CLI (`aibox-cli`, saved in `.aibox/<profile>/api-key`, mode 0600); an ExternalSecret (creation and deletion policy `Merge`) adds the operator's keys from the secret store (`aibox/<cluster>/api-keys`, a JSON object) and tolerates their absence. The client id is forwarded as `x-aibox-client` for rate limiting and logs. (decided by agent: no keys in git; the platform works before any key is stored)
- Q: How are rate limits configured and enforced? → A: Profile field `spec.gateway.rateLimits` with a default and per-client `requestsPerMinute` and `tokensPerMinute`; rendered into a `BackendTrafficPolicy` with global rules keyed on `x-aibox-client` (requests) and a token-cost rule fed by the AI Gateway's `llmRequestCosts` (total tokens of each response). Excess requests get 429; if Redis is unavailable, Envoy's rate-limit service fails open (documented). (decided by agent: limits belong to the environment, so the profile carries them)
- Q: TLS? → A: AWS: the NLB terminates TLS with the ACM certificate passed as `aibox up --tls-cert-arn` (root Application parameter; the ARN contains the account id, so it never goes to git) and is restricted to the admin CIDRs; without a certificate `up` warns that the endpoint is plain HTTP. Local: plain HTTP on 127.0.0.1:30080. (decided by agent: no cert-manager ACME dependency; ACM is the AWS default)
- Q: How do smoke and the metrics contract keep reaching the model server's `/metrics`? → A: Smoke sends its requests through the gateway with the CLI key; the model servers' metrics move to a NodePort 30082 (`inference-metrics`, local only; kind maps it to 127.0.0.1). (decided by agent: the gateway exposes only the OpenAI API)
- Q: Body size limit? → A: 4 MiB request bodies (`ClientTrafficPolicy`); larger bodies get 413. (decided by agent: comfortably above a 128k-token prompt)

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
