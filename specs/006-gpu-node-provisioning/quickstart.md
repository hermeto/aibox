# Quickstart: 006

```bash
go run ./cmd/aibox render nodepools --profile aws-bench
make generate && git diff --exit-code platform/gpu/
make validate-manifests
go run ./cmd/aibox up --profile local && kubectl get nodes -L aibox.dev/gpu-class
```
