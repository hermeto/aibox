# Data model: 009

- **Profile** (`schemas/profile.schema.json`): `spec.gateway.rateLimits.default` and
  `spec.gateway.rateLimits.clients.<client-id>`, each `{requestsPerMinute, tokensPerMinute}`.
- **Catalog**: optional `routing: round-robin` (default; `epp` added in 011).
- **API keys**: Secret `aibox-api-keys` in `aibox-gateway` (data: client id → key).
- **Rendered per model**: headless Service `<name>-rr`, `Backend` `<name>`,
  `AIServiceBackend` `<name>`; one `AIGatewayRoute` `aibox`.
