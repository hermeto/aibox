# Research: 003 local-platform

## R1 — kind

- kind v0.33.0 (latest tag); default node image
  `kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5`
  (from `pkg/apis/config/defaults/image.go` at v0.33.0).
- `extraPortMappings` with `listenAddress: 127.0.0.1` expose NodePorts 30080/30081 on the host.
- Source: <https://kind.sigs.k8s.io/docs/user/configuration/>

## R2 — llm-d inference simulator v1.1.0

Verified in the v1.1.0 README, `docs/configuration.md`, `docs/metrics.md`, `docs/api.md`:

- Image `ghcr.io/llm-d/llm-d-inference-sim:v1.1.0`, index digest
  `sha256:012cf6aa414cfbb910743274458dd62c0c0a9d469eb2ba2b28fd5b963516447d`, platforms
  linux/amd64 and linux/arm64.
- Flags used: `--model`, `--served-model-name`, `--port`, `--mode random`,
  `--time-to-first-token`, `--inter-token-latency` (duration format), `--max-num-seqs`,
  `--max-model-len`, `--seed`, `--enable-kvcache`, `--kv-cache-size`, `--block-size`. Dummy model
  names use the simulated tokenizer (no HuggingFace download, no render sidecar).
- Endpoints: `/v1/chat/completions`, `/v1/completions`, `/v1/models`, `/metrics`, `/health`,
  `/health/ready`.
- Metrics (subset of vLLM): `vllm:num_requests_running`, `vllm:num_requests_waiting`,
  `vllm:kv_cache_usage_perc`, `vllm:time_to_first_token_seconds`,
  `vllm:inter_token_latency_seconds`, `vllm:e2e_request_latency_seconds`,
  `vllm:prompt_tokens_total`, `vllm:generation_tokens_total`, `vllm:prefix_cache_hits_total`,
  `vllm:prefix_cache_queries_total`, `vllm:request_success_total`.
- Sources: <https://github.com/llm-d/llm-d-inference-sim/blob/v1.1.0/README.md>,
  <https://github.com/llm-d/llm-d-inference-sim/blob/v1.1.0/docs/metrics.md>

Smoke asserts the four series later features depend on:
`vllm:num_requests_running`, `vllm:num_requests_waiting`, `vllm:kv_cache_usage_perc`,
`vllm:time_to_first_token_seconds` (histogram, so `_bucket` lines).

## R3 — Argo CD

- Chart `argo-cd` 10.12.1 (`argoproj/argo-helm` tag `argo-cd-10.12.1`), appVersion v3.6.1.
- Values used (checked in the chart's `values.yaml` at that tag): `global.image.tag`,
  `configs.params."server.insecure"`, `configs.cm."kustomize.buildOptions"`,
  `configs.cm."resource.customizations.health.argoproj.io_Application"`,
  `configs.cm."timeout.reconciliation"`, `server.service.type/nodePortHttp`, `dex.enabled`,
  `notifications.enabled`, `redis.image`.
- Image digests: `quay.io` is not reachable from the agent's build environment, so the
  `quay.io/argoproj/argocd:v3.6.1` digest is read from the first `e2e-local` run
  (`kubectl get pods -o jsonpath='{..imageID}'`, printed by the job) and pinned in
  `platform/bootstrap/argocd/values-local.yaml`. Redis `8.6.4-alpine` index digest
  `sha256:2cc044fc5a07c9b701f8f1255a309ae9ad7856e694ac03513bf3648c01e40763` (Docker official
  image; the ECR Public mirror serves the same index).
- Application health in app-of-apps: Argo CD ≥ 1.8 removed the built-in health assessment of
  `argoproj.io/Application`; the documented Lua customization restores it so sync waves wait:
  <https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/>
- Kustomize + Helm in Argo CD: `kustomize.buildOptions: --enable-helm`:
  <https://argo-cd.readthedocs.io/en/stable/user-guide/kustomize/>

## R4 — Waiting via the Kubernetes API

- Dynamic client on `argoproj.io/v1alpha1, Resource=applications` in namespace `argocd`;
  ready when `status.sync.status == Synced` and `status.health.status == Healthy` for the
  root and every child; poll every 5 s with a deadline; on timeout list each non-ready
  Application with `status.conditions[].message` and `status.operationState.message`.
- Deployments readiness: `status.observedGeneration >= metadata.generation` and
  `status.updatedReplicas == status.availableReplicas == spec.replicas`.

## R5 — Git source detection

- `git remote get-url origin` → normalize `git@github.com:o/r(.git)` and
  `ssh://git@github.com/o/r` to `https://github.com/o/r.git`; `git rev-parse --abbrev-ref HEAD`
  for the branch; `git rev-list --count @{u}..HEAD` > 0 → warning about unpushed commits.

## Decisions

| ID | Decision | Alternatives rejected |
| --- | --- | --- |
| D1 | Helm chart app-of-apps in `platform/bootstrap` | ApplicationSet git generator (harder to parameterize per run); Kustomize root with patches (less common) |
| D2 | NodePort + kind port mappings | `kubectl port-forward` processes (fragile, need lifecycle management) |
| D3 | Argo CD installed by `helm upgrade --install` (no `--wait`), readiness via client-go | `--wait` semantics changed in Helm 4; client-go wait is version-independent |
| D4 | Simulator as plain Kustomize (no chart) | upstream chart not published; manifest is ~60 lines |
| D5 | e2e in Go with the CLI as a library (`internal/platform`) | shell script (no assertions, harder to maintain) |
