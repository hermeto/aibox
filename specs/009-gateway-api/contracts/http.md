# Contract: gateway HTTP API

- `POST /v1/chat/completions`, `POST /v1/completions`, `GET /v1/models` with
  `Authorization: Bearer <key>`; local base URL `http://127.0.0.1:30080/v1`.
- 401 without or with an unknown key; 404 `{"error":{"type":"invalid_request_error","code":"model_not_found",…}}`
  for unknown models; 429 above the key's limits; 413 above 4 MiB bodies.
- `aibox smoke` reads the CLI key from `.aibox/<profile>/api-key`.
