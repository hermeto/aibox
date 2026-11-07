# Agent progress — AIBox

> A fresh session resumes from this file alone. Read `docs/agent/PLAYBOOK.md` for the
> procedure and `CLAUDE.md` for the rules.

## Current state

| Field | Value |
| --- | --- |
| Current feature | 010-observability-stack (implementation complete; PR #12 draft) |
| Spec Kit phase | implement done; CI fixes pushed, waiting for e2e-local on #11 and #12 |
| Last commit | see `git log -1` on `010-observability-stack` |
| Run status | in progress |

## Open PRs (merge order, bottom-up)

1. [#1](https://github.com/hermeto/aibox/pull/1) `001-repo-foundation` → `main`
2. [#4](https://github.com/hermeto/aibox/pull/4) `002-cli-skeleton` → `001-repo-foundation`
3. [#5](https://github.com/hermeto/aibox/pull/5) `003-local-platform` → `002-cli-skeleton`
4. [#6](https://github.com/hermeto/aibox/pull/6) `004-aws-infrastructure` → `003-local-platform`
5. [#7](https://github.com/hermeto/aibox/pull/7) `005-cost-guardrails` → `004-aws-infrastructure`
6. [#8](https://github.com/hermeto/aibox/pull/8) `006-gpu-node-provisioning` → `005-cost-guardrails`
7. [#9](https://github.com/hermeto/aibox/pull/9) `007-model-catalog` → `006-gpu-node-provisioning`
8. [#10](https://github.com/hermeto/aibox/pull/10) `008-inference-runtime` → `007-model-catalog` (draft; CI green)
9. [#11](https://github.com/hermeto/aibox/pull/11) `009-gateway-api` → `008-inference-runtime` (draft; rate-limit policy namespace fixed, CI re-running)
10. [#12](https://github.com/hermeto/aibox/pull/12) `010-observability-stack` → `009-gateway-api` (draft; Loki and lint fixes pushed, CI re-running)

## Blockers / `needs-human`

- [#2](https://github.com/hermeto/aibox/issues/2) one-time repository setup: default branch is
  now `main`; topics, ruleset, labels and milestones still need `hack/github/setup-repo.sh`.
- #11 and #12 e2e-local: last runs failed because the serving overlays overrode the
  rate-limit BackendTrafficPolicy namespace (policy never attached, Argo CD kept `serving`
  Progressing) and Loki crash-looped on a read-only `/var/loki`. Both fixed; confirm the
  re-runs before marking ready.

Tracking issue: [#3 v1.0 roadmap](https://github.com/hermeto/aibox/issues/3).

## Environment (prerequisites check, playbook §3)

The run executes on the maintainer's Mac. Docker must be running for `e2e-local`.

| Tool | Version | How installed |
| --- | --- | --- |
| Go | 1.27.3 | Homebrew |
| kustomize | 5.8.3 | `bin/kustomize` (`make tools`) |
| helm | 4.3.1 | Homebrew |
| kind / kubectl | 0.33.0 / 1.37.2 | Homebrew |
| promtool | 3.16.0 | Homebrew |
| golangci-lint, actionlint, kubeconform | Makefile pins | `make tools` |
| GitHub access | `hermeto` via `gh` | authenticated |

## Run decisions (taken by the agent on the maintainer's behalf)

1. **Feature 010 metrics path.** Prometheus scrapes metrics directly through
   ServiceMonitors/PodMonitors; the OpenTelemetry Collector handles traces and logs only
   (recorded in ADR-0011 and the spec clarifications).
2. **Dashboard lint.** The upstream `grafana/dashboard-linter` cannot be installed with
   `go install` (its module has `replace` directives), so the dashboard model is linted by a
   Go test in `internal/observability/dashboards_test.go`; CI runs it through
   `make lint-dashboards`. Same invariants: valid JSON, unique UID/panel ids, valid grid,
   every panel has a query.
3. **Local access.** Grafana 30300, Prometheus 30301 and Tempo 30302 are NodePorts on the
   local overlay only; cloud overlays stay ClusterIP and are reached by port-forward.
4. **Enabled AWS tracing sampling at 10%** through the EnvoyProxy overlay patch; local samples
   100%.
5. **Local Docker was not available in the implementation session**, so the e2e assertions
   are verified by CI rather than locally; unit tests, rule tests and manifest rendering were
   run locally.
6. **Loki storage.** Loki keeps 24 h of logs on an `emptyDir` at `/var/loki` instead of a PVC;
   logs are lost on pod restart, which is acceptable for the retention target.
7. **Serving overlay namespaces.** The serving overlays no longer set an overlay-wide
   `namespace:`; generated objects carry explicit namespaces and `test/gateway` guards that
   Gateway-targeting policies stay in `aibox-gateway`.
