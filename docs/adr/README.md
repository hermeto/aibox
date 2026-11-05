# Architecture Decision Records

Decisions are recorded with the MADR-based [template](template.md). Numbers are never reused.

| ADR | Title | Status | Decided by |
| --- | --- | --- | --- |
| [0000](0000-record-architecture-decisions.md) | Record architecture decisions | Accepted | Feature 001 |
| [0001](0001-eks-karpenter-substrate.md) | EKS + Karpenter as the reference substrate | Accepted | Feature 004 |
| [0002](0002-kserve-llminferenceservice.md) | KServe LLMInferenceService as the per-model serving resource | Accepted | Feature 008 |
| [0003](0003-gateway-implementation.md) | Envoy Gateway with the Envoy AI Gateway as the inference gateway | Accepted | Feature 009 |
| [0007](0007-weight-loading-and-cold-start.md) | Weight loading and cold start | Proposed — pending E7 | Experiment E7 |
| [0008](0008-cli-in-go-with-cobra.md) | CLI in Go with Cobra | Accepted | Feature 002 |
| [0009](0009-gitops-with-argocd-app-of-apps.md) | GitOps with Argo CD app-of-apps | Accepted | Feature 003 |
| [0010](0010-mandatory-budget-and-ttl.md) | Mandatory budget and TTL for cloud runs | Accepted | Feature 005 |

Planned ADRs are listed in the product spec, [§A.10](../product/spec.md#a10-adr-index-planned).
