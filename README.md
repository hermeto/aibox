# AIBox — AI Platform in a Box

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

> 🚧 **Under construction** — see the [v1.0 roadmap issue](https://github.com/hermeto/aibox/issues?q=is%3Aissue+%22v1.0+roadmap%22) for progress and merge order.

AIBox is a reference LLM inference platform on Kubernetes that comes up with one
command and ships a reproducible benchmark report: cost per 1M tokens, p99 time to
first token (TTFT), and the measured effect of every architecture decision. It
combines vLLM/SGLang, llm-d, the Gateway API Inference Extension, KEDA, Karpenter and
OpenTelemetry, and runs locally on kind (no GPU) as well as on AWS EKS.

- Product specification: [`docs/product/spec.md`](docs/product/spec.md)
- Project constitution: [`.specify/memory/constitution.md`](.specify/memory/constitution.md)

## License

Apache-2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).
