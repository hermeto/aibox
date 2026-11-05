# Quickstart: 005 cost-guardrails

```bash
go test ./internal/cost/... ./internal/platform/... -race -cover
make terraform-check            # includes infra/terraform/aws/guardrails
BUDGET_USD=50 ./bin/aibox up --profile aws-dev --notify-email you@example.com   # maintainer only
./bin/aibox ttl extend 2h --profile aws-dev
./bin/aibox cost --profile aws-dev
./bin/aibox down --profile aws-dev
```
