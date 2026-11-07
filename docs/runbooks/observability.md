# Observability runbook

The `observability` Argo CD component ships Prometheus, Alertmanager, Grafana, Tempo and
Loki, plus two OpenTelemetry Collector workloads ([ADR-0011](../adr/0011-prometheus-metrics-backend.md)).
Everything is provisioned from git: dashboards, data sources, recording rules and alerts.

## Endpoints

| Profile | Grafana | Prometheus | Tempo (HTTP) |
| --- | --- | --- | --- |
| `local` | <http://127.0.0.1:30300> | <http://127.0.0.1:30301> | <http://127.0.0.1:30302> |
| `aws-*` | `kubectl -n observability port-forward svc/monitoring-grafana 3000:80` | `kubectl -n observability port-forward svc/monitoring-kube-prometheus-prometheus 9090:9090` | `kubectl -n observability port-forward svc/tempo 3200:3200` |

Grafana has no default password: the chart generates one into the `monitoring-grafana`
Secret. Read it with:

```bash
kubectl -n observability get secret monitoring-grafana \
  -o jsonpath='{.data.admin-password}' | base64 -d; echo
```

The user is `admin`.

## What is collected

| Signal | Source | Destination |
| --- | --- | --- |
| Runtime metrics | `PodMonitor/model-servers` → `/metrics` on the model servers | Prometheus |
| Gateway metrics | `PodMonitor/envoy-gateway` → `/stats/prometheus` | Prometheus |
| GPU metrics | `ServiceMonitor/dcgm-exporter` (cloud only) | Prometheus |
| KEDA and Karpenter | `ServiceMonitor/keda`, `PodMonitor/karpenter` (cloud only) | Prometheus |
| Traces | Envoy Gateway OTLP → `otel-traces` Deployment (100% local, 10% AWS) | Tempo |
| Logs | `otel-logs` DaemonSet (`filelog` + Kubernetes metadata) | Loki |

Monitors whose workload is absent (DCGM, KEDA and Karpenter on `local`) are inert.

## Rules and dashboards

`make generate` regenerates `platform/observability/rules/<profile>.rules.yaml` and the
`PrometheusRule` in each overlay from the catalog, the price table and
`metrics/runtime-metrics.yaml`. Do not edit them by hand.

- Canonical series: `aibox:requests_waiting`, `aibox:kv_cache_usage_ratio`,
  `aibox:time_to_first_token_seconds:p99`, `aibox:*_tokens_total:rate1m`, `aibox:*cost*`.
- Alerts: TTFT p99 above the catalog SLO for 10 minutes, KV-cache above 90% for 5 minutes,
  pods pending for 5 minutes, a GPU node idle for 30 minutes, and (cloud) a price missing for
  an instance type.
- Dashboards: SLO per model, capacity, GPU, cost, scaling. They are ConfigMaps labelled
  `grafana_dashboard=1`; the Grafana sidecar loads them from every namespace.

Verify the rules and dashboards locally:

```bash
make test-rules          # promtool test rules
make lint-dashboards     # grafana/dashboard-linter
make generate && git diff --exit-code platform/observability
```

## Common problems

**No runtime series in Prometheus.** Check that the model server pod carries
`app.kubernetes.io/part-of: aibox` and exposes `/metrics` on the named port `http`; then
check `PodMonitor/model-servers` and the Prometheus targets page.

**The TTFT alert does not fire.** It needs the canonical recording rule, so at least one
minute of samples after traffic. The target comes from `slo.ttftP99Ms` in
`models/catalog.yaml`; change the catalog, not the rule.

**No traces.** Locally the simulator does not emit spans; the trace comes from Envoy
Gateway. Send one request through the gateway, then search Tempo for the `aibox-gateway`
service. On AWS, confirm the `EnvoyProxy` telemetry provider resolves
`otel-traces.observability.svc.cluster.local:4317`.

**Cost panels are empty.** Cost rules exist only for cloud profiles (there is no GPU node
locally). On AWS, run `aibox render rules --profile aws-dev` and check that the node's
instance type is present in `data/pricing/aws-<region>.yaml`; `AIBoxPriceMissing` warns when
it is not.

**Grafana restarted and lost a dashboard.** Dashboards are mounted from ConfigMaps; the
sidecar reloads them on change. If a dashboard is absent, verify the ConfigMap label and the
`grafana_dashboard` sidecar logs.

## Retention and cost

Every backend keeps 24 h (metrics, traces, logs) and uses local storage on `local` and
whatever the storage class provides on AWS. Benchmark runs export their own snapshots, so
24 h is enough and keeps the footprint small. Enabling Tempo metrics-generator or long-term
storage is a v2 decision.
