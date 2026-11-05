# AIBox — AI Platform in a Box

[![CI](https://github.com/hermeto/aibox/actions/workflows/ci.yml/badge.svg)](https://github.com/hermeto/aibox/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/hermeto/aibox/badge)](https://scorecard.dev/viewer/?uri=github.com/hermeto/aibox)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

> 🚧 **Under construction** — see the [v1.0 roadmap issue](https://github.com/hermeto/aibox/issues?q=is%3Aissue+%22v1.0+roadmap%22) for progress and merge order.

AIBox is a reference LLM inference platform on Kubernetes that comes up with one
command and ships a reproducible benchmark report: cost per 1M tokens, p99 time to
first token (TTFT), and the measured effect of every architecture decision. It
combines vLLM/SGLang, llm-d, the Gateway API Inference Extension, KEDA, Karpenter and
OpenTelemetry, and runs locally on kind (no GPU) as well as on AWS EKS.

- Product specification: [`docs/product/spec.md`](docs/product/spec.md)
- Project constitution: [`.specify/memory/constitution.md`](.specify/memory/constitution.md)
- Architecture decisions: [`docs/adr/`](docs/adr/README.md)
- Feature specs (Spec Kit): [`specs/`](specs/)

## 5-minute local quickstart

Runs the whole platform on [kind](https://kind.sigs.k8s.io/) with no GPU and no cloud
account: a simulated model (`sim-llama-8b`, served by the
[llm-d inference simulator](https://github.com/llm-d/llm-d-inference-sim)) behind an
OpenAI-compatible API, delivered by Argo CD.

Prerequisites: Docker (4 CPUs / 8 GB for the Docker VM), `kind`, `kubectl`, `helm`, Go.

```bash
git clone https://github.com/hermeto/aibox && cd aibox
make build
./bin/aibox doctor --profile local      # checks tools and Docker resources
./bin/aibox up --profile local          # kind + Argo CD + app-of-apps (~5 minutes)
curl -s http://127.0.0.1:30080/v1/chat/completions -H 'Content-Type: application/json' \
  -d '{"model":"sim-llama-8b","messages":[{"role":"user","content":"Hello"}]}'
./bin/aibox smoke --profile local       # models, one completion per model, metrics
./bin/aibox models sync --profile local --model tiny-random-llama  # mirror + verify weights
./bin/aibox down --profile local        # deletes the cluster
```

Argo CD tracks your current branch on `origin`; push first, or use `--from-worktree` to apply
unpushed changes. The Argo CD UI is at <http://127.0.0.1:30081> (user `admin`).

## Cloud profiles (AWS)

`aibox up --profile aws-dev` provisions VPC, EKS, IAM (Pod Identity) and an encrypted weights
bucket with Terraform/OpenTofu (`infra/terraform/aws`), then bootstraps Argo CD. **It costs
money** — follow [docs/runbooks/aws-first-run.md](docs/runbooks/aws-first-run.md) (quotas,
state bucket, first run, teardown) and always finish with `aibox down --profile aws-dev`.
Cloud runs require a budget (`BUDGET_USD`) and tear themselves down when their TTL expires
([cost guardrails](docs/runbooks/cost-guardrails.md)).

## Models

`models/catalog.yaml` declares every model AIBox serves, pinned to a Hugging Face commit, with
its runtime, GPU class, tensor parallelism, loading method and SLO targets
(`aibox models list`). `aibox models sync` mirrors the weights into the environment's object
storage with in-cluster Jobs and verifies every checksum; see
[docs/runbooks/model-weights.md](docs/runbooks/model-weights.md).

## The `aibox` CLI

```bash
make build                                   # ./bin/aibox
./bin/aibox --help                           # up, down, models, smoke, bench, report, cost, doctor, version
./bin/aibox doctor --profile local           # preflight checks (tools, Docker resources)
./bin/aibox doctor --profile aws-dev -o json # read-only AWS checks; skipped without credentials
make up PROFILE=local                        # make targets are thin wrappers around the CLI
```

Profiles live in [`profiles/`](profiles/) and are validated against
[`schemas/profile.schema.json`](schemas/profile.schema.json). Exit codes: 0 ok, 1 failure,
2 not implemented yet, 64 usage error.

## Contributing

Start with [CONTRIBUTING.md](CONTRIBUTING.md) — `make dev-setup && make lint test` is all you
need, no cloud credentials. Please read the [Code of Conduct](CODE_OF_CONDUCT.md).

| Topic | File |
| --- | --- |
| Getting help | [SUPPORT.md](SUPPORT.md) |
| Reporting vulnerabilities | [SECURITY.md](SECURITY.md) |
| Governance and maintainers | [GOVERNANCE.md](GOVERNANCE.md), [MAINTAINERS.md](MAINTAINERS.md), [CODEOWNERS](.github/CODEOWNERS) |
| Pull request template | [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md) |
| Repository settings as code | [ruleset](.github/rulesets/main.json), [labels](.github/labels.yml), [setup script](hack/github/setup-repo.sh) |

## License

Apache-2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).
