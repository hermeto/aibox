# Contract: `aibox render serving`

- `aibox render serving --profile <p> [--catalog file]` — prints the LLMInferenceServices
  (and the fetch-scripts ConfigMap when a model uses `nvme-cache`) for the profile's enabled,
  served models; exit 1 when a model cannot be placed (TP > GPUs) or a combination is
  unsupported. `make generate` writes `platform/serving/overlays/<p>/models.yaml`.
- Metric contract: `go test ./test/contract/... -run TestRuntimeMetrics` (live check with
  `AIBOX_METRICS_URL=http://127.0.0.1:30080/metrics`).
