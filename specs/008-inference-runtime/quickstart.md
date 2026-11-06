# Quickstart: 008

```bash
go run ./cmd/aibox render serving --profile aws-bench | less
make generate && git diff --exit-code platform/serving
go test ./internal/serving/... ./test/contract/...
go run ./cmd/aibox up --profile local && go run ./cmd/aibox smoke --profile local
AIBOX_METRICS_URL=http://127.0.0.1:30080/metrics go test ./test/contract/... -run TestRuntimeMetrics
```
