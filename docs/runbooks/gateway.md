# Runbook: inference gateway

One OpenAI-compatible endpoint per environment ([ADR-0003](../adr/0003-gateway-implementation.md)):
Envoy Gateway with the Envoy AI Gateway, API keys and per-key rate limits.

| Profile | Endpoint | TLS |
| --- | --- | --- |
| local | `http://127.0.0.1:30080/v1` | none (localhost only) |
| aws-* | `https://<load balancer>/v1` (port 443) | ACM certificate from `aibox up --tls-cert-arn`; without it, plain HTTP on port 443 |

On AWS the load balancer accepts only the admin CIDRs (`--admin-cidr`, default your egress IP).

## API keys

Send `Authorization: Bearer <key>`. Keys live in the Secret `aibox-api-keys`
(`aibox-gateway` namespace; data: client id -> key):

- `aibox up` creates the CLI's key (client `aibox-cli`) and saves it in
  `.aibox/<profile>/api-key` (mode 0600); `aibox smoke` and `aibox bench` use it.
- Operator keys come from the secret store: put a JSON object of client id -> key in
  AWS Secrets Manager under `aibox/<cluster>/api-keys`; External Secrets merges it into the
  Secret within an hour.

```bash
aws secretsmanager create-secret --region us-east-1 --name aibox/aibox-bench/api-keys \
  --secret-string '{"team-a":"<a long random key>"}'
```

Requests without a key, or with an unknown key, get 401. Never commit keys.

## Rate limits

`spec.gateway.rateLimits` in the profile sets requests and tokens per minute: `default` for
every client (each with its own counter) and `clients.<id>` for specific clients. Tokens are
the total tokens the runtime reports in each response, so a request that crosses the limit
completes and the following ones get 429 until the minute ends. Counters live in Redis
(`envoy-gateway-system/redis`); if Redis is down, requests are allowed (fail open).

## Routing

The `model` field of the body selects the route (`AIGatewayRoute aibox` in `aibox-serving`,
rendered from the catalog). `routing: round-robin` (default) spreads requests across the
model's servers through a headless Service. Unknown models get 404 with
`"code": "model_not_found"`; bodies above 4 MiB get 413.

## Troubleshooting

```bash
kubectl get gateway -n aibox-gateway aibox -o yaml
```

```bash
kubectl get aigatewayroute,httproute -A
```

```bash
kubectl -n envoy-gateway-system logs deploy/envoy-gateway --tail=100
```

| Symptom | Cause | Fix |
| --- | --- | --- |
| 401 with a valid key | Secret not reconciled yet, or key not in `aibox-api-keys` | check the Secret's data; wait for the ExternalSecret refresh |
| 404 for a served model | model not ready, or not in the route | `kubectl -n aibox-serving get llminferenceservices`; [model-load-failures.md](model-load-failures.md) |
| 429 below the limit | another client shares the id, or the minute window started earlier | check `spec.gateway.rateLimits`; counters reset every minute |
| 503 | no ready model server behind the route | same as 404 |
