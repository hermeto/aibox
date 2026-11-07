# Quickstart: 010

```bash
go run ./cmd/aibox up --profile local
open http://127.0.0.1:30300   # Grafana; password: see docs/runbooks/observability.md
promtool test rules platform/observability/rules/tests/*.yaml
```
