# Feature Specification: Observability stack — metrics, traces, logs, dashboards, alerts and live cost

**Feature Branch**: `010-observability-stack`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "Observability as a product feature. Every claim in the benchmark report must be visible live in dashboards, and every request must be traceable from gateway to decode; cost per 1M tokens visible in real time. OpenTelemetry Collector (agent + gateway) for metrics, traces and logs; Prometheus via kube-prometheus-stack, Tempo, Loki, Grafana with datasources and dashboards from git; five dashboards (SLO per model, capacity, GPU, cost, scaling); alerts as code (TTFT p99 SLO from the catalog, KV-cache > 90% for 5 min, pods pending > 5 min, GPU node idle > 30 min); cost recording rules from the price table; traces gateway → routing → runtime where supported (playbook §6, feature 010)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Metrics, traces and logs land in one place (Priority: P1)

The platform collects the metrics of the model servers, the gateway, the Endpoint Picker,
GPUs (DCGM), KEDA and Karpenter, the traces of requests and the logs of every pod, and keeps
them in Prometheus, Tempo and Loki with short retention.

**Why this priority**: the benchmark report and every experiment read these signals.

**Independent Test**: in CI on kind, the simulator's metrics appear in Prometheus, a smoke
request's trace appears in Tempo and the simulator's logs appear in Loki.

**Acceptance Scenarios**:

1. **Given** the local platform, **When** smoke runs, **Then** Prometheus has the runtime
   series of the local model and Tempo has a trace for one of its requests.

---

### User Story 2 - Five dashboards from git (Priority: P1)

Grafana ships, from git, the dashboards SLO per model (TTFT, ITL, end-to-end p50/p95/p99,
error rate), capacity (queue, KV-cache usage, running batch, prefix-cache hit rate), GPU
(SM utilization, memory, power), cost (USD per hour per pool, USD per 1M output tokens) and
scaling (replicas, nodes, provisioning phase timings). They work for vLLM and SGLang through
the runtime metric mapping.

**Independent Test**: dashboards pass a linter in CI and load without errors through the
Grafana API in e2e-local.

---

### User Story 3 - Alerts as code (Priority: P1)

Alert rules fire on: TTFT p99 above the model's catalog target, KV-cache above 90% for
5 minutes, pods pending for more than 5 minutes, and a GPU node idle for more than 30 minutes.

**Independent Test**: rule unit tests (`promtool test rules`) cover firing and non-firing
cases.

---

### User Story 4 - Live cost (Priority: P1)

Recording rules combine each node's instance type and capacity type with the price table
(feature 005) into USD per hour per pool and USD per 1M output tokens.

**Independent Test**: rule unit tests compute the expected cost for a synthetic node set.

---

### User Story 5 - Request traces (Priority: P2)

A request is traceable from the gateway through the routing decision to the model server
where the pinned versions support it; gaps are documented.

### Edge Cases

- No GPUs locally → GPU panels show "no data"; no alert fires.
- High-cardinality labels → no per-request labels; request ids only in traces and logs.
- Price missing for an instance type → the cost series is absent for that pool and a
  warning-level alert reports it.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: An OpenTelemetry Collector MUST collect traces (OTLP) and logs and forward them to
  Tempo and Loki; Prometheus MUST scrape the runtime, gateway, router, DCGM, KEDA and
  Karpenter metrics.
- **FR-002**: Grafana MUST be provisioned from git with Prometheus, Tempo and Loki data sources
  and the five dashboards, using canonical metric names via the runtime mapping.
- **FR-003**: Alert rules MUST cover the four conditions; the TTFT SLO MUST come from the catalog.
- **FR-004**: Cost recording rules MUST be generated from the price table, not hand-written.
- **FR-005**: Rules MUST have unit tests run in CI; dashboards MUST be linted in CI.
- **FR-006**: The local profile MUST run the same stack with small resources; Grafana at
  127.0.0.1:30300.

## Success Criteria *(mandatory)*

- **SC-001**: e2e-local asserts simulator metrics in Prometheus, a smoke trace in Tempo and
  dashboards loading through the Grafana API.
- **SC-002**: `promtool test rules` passes for every rule file in CI.
- **SC-003**: Dashboards pass the linter in CI.

## Assumptions

- Long-term storage (Mimir, Thanos) and SaaS backends are out of scope.
- Retention: 24 h for metrics, traces and logs (benchmark runs export their own results).
