# Research: 009 gateway-api

- Envoy Gateway v1.9.3 (latest tag 2026-11-05). `SecurityPolicy.apiKeyAuth`
  (`api/v1alpha1/api_key_auth_types.go`): `credentialRefs` (Secrets: key = client id,
  value = API key), `extractFrom[].headers`, `forwardClientIDHeader`, `sanitize`. Envoy's
  `api_key_auth` filter strips a `Bearer ` prefix from the header value
  (`api/envoy/extensions/filters/http/api_key_auth/v3/api_key_auth.proto`), so OpenAI clients'
  `Authorization: Bearer <key>` works. Rate limits (`ratelimit_types.go`): `global`/`local`
  rules with `clientSelectors`, `limit`, `cost` (request/response, from metadata) and `shared`;
  global rules need the rate-limit service with a Redis backend.
- Envoy AI Gateway v1.2.0 (latest; built on Envoy Gateway v1.9.2, Gateway API v1.6.1, GIE
  v1.6.2): `AIGatewayRoute` (`aigateway.envoyproxy.io/v1beta1`) with `parentRefs`, `rules[]`
  (`matches[].headers`, `backendRefs[]` to `AIServiceBackend` or `InferencePool`,
  `timeouts`, `modelsOwnedBy`), `llmRequestCosts`; `AIServiceBackend.backendRef` must be an
  Envoy Gateway `Backend` (validation rule in `api/v1beta1/ai_service_backend.go`). Required
  Envoy Gateway settings: `manifests/envoy-gateway-values.yaml` (extension manager hooks,
  `enableBackend`, `enableEnvoyPatchPolicy`); token rate limiting adds the Redis backend
  (`examples/token_ratelimit/envoy-gateway-values-addon.yaml`).
- KServe v0.21.0's installer uses Envoy Gateway v1.8.1 and Envoy AI Gateway v1.1.0 with
  Gateway API v1.5.1 — the same pairing one minor older.
