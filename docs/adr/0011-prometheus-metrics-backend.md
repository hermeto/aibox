# ADR-0011: Prometheus (kube-prometheus-stack) as the metrics backend

- **Status**: Accepted
- **Date**: 2026-11-05
- **Deciders**: AIBox maintainers (proposed by the agent in feature 010)
- **Decided by**: feature 010

## Context and problem statement

The report and every experiment need the signals of the platform: runtime queue depth,
KV-cache usage, TTFT/ITL histograms, gateway rates, DCGM GPU metrics, KEDA and Karpenter
state, and live cost per 1M tokens. They must be queryable with the PromQL that the
dashboards, alerts, autoscaling policies and the benchmark collector share, from the same
stack on kind and on EKS, and provisioned from git.

## Decision drivers

- One query language and one set of recording rules for dashboards, alerts and KEDA.
- ServiceMonitor/PodMonitor/PrometheusRule CRDs that the GPU operator, KEDA, Karpenter and
  Envoy Gateway already integrate with.
- Runs on kind with small resources; no managed service required.
- Everything (datasources, dashboards, rules) reviewable in git.

## Considered options

1. **kube-prometheus-stack** (Prometheus Operator, Prometheus, Alertmanager, Grafana,
   kube-state-metrics, node-exporter).
2. Plain Prometheus StatefulSet with hand-written scrape config.
3. VictoriaMetrics or Mimir with a compatible query API.
4. A managed SaaS backend.

## Decision outcome

Chosen option: "kube-prometheus-stack", because it is the community reference deployment,
already brings the operator CRDs that every other AIBox component integrates with, and
keeps the whole stack inside the cluster at zero cost on kind.

Consequences:

- The `observability` Argo CD component owns Prometheus, Alertmanager, Grafana, Tempo, Loki
  and two OpenTelemetry Collector workloads. It is deployed at sync wave −1, before model
  servers, so their PodMonitor exists when the first pod starts.
- Metrics are scraped directly by Prometheus through ServiceMonitors/PodMonitors; the
  Collector handles traces and logs only. This is a deliberate deviation from "the
  Collector collects metrics": it removes one hop, one failure mode and one copy of every
  metric, and the pinned Collector is not needed for scraping.
- Rules are generated: canonical runtime series from `metrics/runtime-metrics.yaml`, cost
  from the price table, alerts from the catalog (`aibox render rules`). Dashboards query
  only `aibox:*` series so they work with vLLM and SGLang.
- Retention is 24 h on every backend; benchmark runs export their own results, so long-term
  storage (Mimir/Thanos) is out of scope for v1.

## Evidence

- `platform/observability/base/kustomization.yaml` (pinned chart and images).
- `internal/observability/rules.go` and `platform/observability/rules/tests/` (generated
  rules and their promtool tests).
- `platform/observability/base/dashboards/` (five dashboards, linted in CI).
- `test/e2e/observability_test.go` (simulator series in Prometheus, a gateway trace in
  Tempo, dashboards served by Grafana).
