# Research: 010 observability-stack

- Chart versions (OCI tag listings on ghcr, 2026-11-05): kube-prometheus-stack 92.20.0,
  tempo 1.24.4, loki 7.5.0, grafana 10.5.15 (bundled by kube-prometheus-stack),
  opentelemetry-collector 0.178.2.
- Metric names come from `metrics/runtime-metrics.yaml` (feature 008, verified in the vLLM
  0.33.0 and SGLang 0.5.23 sources); DCGM exporter 4.8.3 exposes `DCGM_FI_DEV_GPU_UTIL`,
  `DCGM_FI_DEV_FB_USED`, `DCGM_FI_DEV_POWER_USAGE`.
- Envoy Gateway tracing: `EnvoyProxy.spec.telemetry.tracing` with an OpenTelemetry provider
  (`backendRefs` to the Collector's OTLP gRPC port) and `samplingRate`.
- kube-state-metrics exposes node labels only when allow-listed
  (`metricLabelsAllowlist: nodes=[...]`), needed to join instance and capacity type with prices.
