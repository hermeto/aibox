# Research: 008 inference-runtime

- KServe v0.21.0 (2026-09-25; latest on PyPI/GitHub tags): LLMInferenceService API
  `serving.kserve.io/v1alpha2` (`pkg/apis/serving/v1alpha2/llm_inference_service_types.go`):
  `spec.model.{uri,name}`, `spec.storageInitializer.enabled`, inline `WorkloadSpec`
  (`replicas` | `scaling`, `parallelism.tensor`, `template` PodSpec, `worker`), `router`
  (route, gateway, scheduler), `prefill`, `baseRefs`. Published charts
  `oci://ghcr.io/kserve/charts/kserve-llmisvc-crd` and `kserve-llmisvc-resources`, v0.21.0.
  Install-script dependency versions at that tag: cert-manager v1.17.0+, Gateway API v1.5.1,
  GIE v1.5.0, LWS v0.8.0, llm-d router v0.10.0, WVA v0.9.0, KEDA 2.20.2.
- Controller (`pkg/controller/v1alpha2/llmisvc/controller.go`): watches HTTPRoute, Gateway,
  InferencePool, ScaledObject, LeaderWorkerSet only when `IsCrdAvailable` — no gateway needed
  for workload-only use; CRDs installed later require a controller restart.
- Default template `config/llmisvcconfig/config-llm-template.yaml`: container `main`
  (`ghcr.io/llm-d/llm-d-cuda:v0.9.0`, `vllm serve /mnt/models --served-model-name … --port 8000`),
  probes on `/health:8000`, `preStop` sleep 15, `terminationGracePeriodSeconds: 60`, volumes
  `home`, `dshm` (1 GiB), `model-cache`, `tmp-dir`, `tls-certs` (secret always reconciled,
  `workload.go`). Merge is a strategic merge patch (`config_merge.go`, `MergeSpecs`).
- TLS: `enableLLMInferenceServiceTLS` in the `ingress` entry of `inferenceservice-config`; the
  chart's `files/common/configmap-patch.yaml` does not set it → HTTP.
- `workload_storage.go`: `storageInitializer.enabled: false` returns before any URI handling.
- vLLM v0.33.0 (2026-11-02): flags in `vllm/engine/arg_utils.py` — `--tensor-parallel-size`,
  `--max-model-len`, `--gpu-memory-utilization`, `--max-num-seqs`, `--kv-cache-dtype`
  (`auto|fp8|fp8_e4m3|fp8_e5m2|…`, `vllm/config/cache.py`), `--[no-]enable-prefix-caching`,
  `--[no-]enable-chunked-prefill` (BooleanOptionalAction), `--load-format runai_streamer`
  (`model_executor/model_loader/__init__.py`), `--quantization fp8|awq_marlin`,
  `--shutdown-timeout` (0 = abort, > 0 = wait). Metrics in `vllm/v1/metrics/loggers.py`:
  `vllm:num_requests_running`, `vllm:num_requests_waiting`, `vllm:kv_cache_usage_perc`,
  `vllm:time_to_first_token_seconds`, `vllm:inter_token_latency_seconds`,
  `vllm:e2e_request_latency_seconds`, `vllm:request_queue_time_seconds`, counters
  `vllm:prompt_tokens`, `vllm:generation_tokens`, `vllm:request_success` (exposed with `_total`).
- SGLang v0.5.23 (2026-10-30; wheel sources): CLI generated from dataclasses
  (`sglang/srt/arg_groups`, `--` + field with dashes): `--model-path`, `--served-model-name`,
  `--host`, `--port`, `--tp-size`, `--context-length`, `--mem-fraction-static`,
  `--max-running-requests`, `--kv-cache-dtype` (`auto|fp8_e5m2|fp8_e4m3|…`),
  `--disable-radix-cache`, `--chunked-prefill-size`, `--enable-metrics`,
  `--load-format runai_streamer`, `--quantization fp8|awq_marlin`. Metrics
  (`sglang/srt/observability`): `sglang:num_running_reqs`, `sglang:num_queue_reqs`,
  `sglang:token_usage`, `sglang:time_to_first_token_seconds`,
  `sglang:inter_token_latency_seconds`, `sglang:e2e_request_latency_seconds`,
  `sglang:prompt_tokens_total`, `sglang:generation_tokens_total`, `sglang:num_requests_total`.
- llm-d inference simulator v1.1.0: vLLM metric names (docs `metrics.md`), `/health`, optional
  TLS flags.
- cert-manager v1.21.3: latest tag; chart `https://charts.jetstack.io`.
