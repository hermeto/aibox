# Data model: 007

See `schemas/catalog.schema.json`. Catalog: `apiVersion: aibox.dev/v1alpha1`,
`kind: ModelCatalog`, `models[]` with `id` (DNS label), `enabled`, `serve` (default true),
`profiles[]`,
`source {type: huggingface|simulator, repo, revision}`, `license`, `gated`, `quantization`
(bf16|fp16|fp8|awq|none), `tensorParallelSize` (1–8), `maxModelLen`, `minGPUClass`,
`runtime` (vllm|sglang|simulator), `runtimeFlags` (prefix caching, chunked prefill,
gpuMemoryUtilization, maxNumSeqs, kvCacheDtype, extraArgs), `loadMethod`
(s3-stream|nvme-cache|hf-download), `slo {ttftP99Ms, itlP99Ms}`.
