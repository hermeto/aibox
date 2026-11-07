# Implementation Plan: Observability stack

**Branch**: `010-observability-stack` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

## Summary

Component `observability` (wave −2): kube-prometheus-stack 92.20.0, Tempo 1.24.4, Loki 7.5.0
and the OpenTelemetry Collector 0.178.2 (traces Deployment, logs DaemonSet), Grafana data
sources for Tempo and Loki, five dashboards from git, and PrometheusRules. `aibox render rules`
generates the canonical runtime series (from `metrics/runtime-metrics.yaml`), the cost rules
(from `data/pricing`) and the SLO alerts (from the catalog) into
`platform/observability/rules/generated.yaml`; static alerts live next to it. Monitors scrape
the model servers, Envoy, DCGM and Karpenter. Envoy Gateway exports traces to the Collector.
Rules are unit-tested with promtool; dashboards are linted; e2e-local checks Prometheus, Tempo
and Grafana.

## Technical Context

**Language/Version**: Go 1.27.3.

**Primary Dependencies**: charts from `oci://ghcr.io/prometheus-community/charts`,
`oci://ghcr.io/grafana/helm-charts`, `oci://ghcr.io/open-telemetry/opentelemetry-helm-charts`;
promtool (Prometheus release, CI), grafana/dashboard-linter (pinned).

**Testing**: rule generation golden/drift tests, `promtool test rules`, dashboard lint, e2e.

## Constitution Check

| Principle | Status | Evidence |
| --- | --- | --- |
| I | Pass | ADR-0011 (Accepted). |
| II | Pass | Charts and images pinned. |
| III | Pass | Same stack on kind with small resources. |
| IV | Pass | Cost visible live; 24 h retention, no extra cloud services. |
| V | Pass | Dashboards, rules and alerts from git. |
| VI | Pass | Rule unit tests, generation drift test, e2e assertions. |
| VII | Pass | This feature. |
| VIII | Pass | No default Grafana password; no per-request labels. |
| IX | Pass | Upstream charts only. |
| X | Pass | ADR-0011, runbook `observability.md`. |

## Project Structure

```text
platform/observability/{base,overlays/local,overlays/aws-dev,overlays/aws-bench}
platform/observability/dashboards/*.json, platform/observability/rules/{alerts.yaml,generated.yaml,tests/}
internal/observability/rules.go (+ tests)   # aibox render rules
test/e2e/observability_test.go
docs/adr/0011-prometheus-metrics-backend.md, docs/runbooks/observability.md
```

## Complexity Tracking

No violations.
