# Quickstart: 007

```bash
go run ./cmd/aibox models list
go run ./cmd/aibox models list --profile aws-bench -o json | jq .
go run ./cmd/aibox models sync --profile local --model tiny-random-llama
go test ./internal/models/... -race
```
