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
