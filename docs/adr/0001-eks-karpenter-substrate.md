# ADR-0001: EKS + Karpenter as the reference substrate (GKE deferred to v2)

- **Status**: Accepted
- **Date**: 2026-11-05
- **Deciders**: maintainer (product spec §A.2 scope); module and topology choices by the agent on the maintainer's behalf
- **Decided by**: feature 004

## Context and problem statement

AIBox needs one reference cloud substrate where GPU capacity (L4, A10G, H100) appears just in
time, is cheap to idle, and every dollar is attributable to a run. The substrate must be
reproducible from code and destroyable with one command.

## Decision drivers

- GPU availability across L4 (g6), A10G (g5) and H100 (p5 with EC2 Capacity Blocks).
- Just-in-time, right-sized GPU nodes (scale-to-zero) — the dominant cost.
- Mature Terraform modules; least-privilege workload identity.
- Tag-based cost attribution and orphan detection.

## Considered options

1. EKS + Karpenter (community Terraform modules, EKS Pod Identity).
2. EKS + Cluster Autoscaler with fixed GPU node groups.
3. GKE (Autopilot or Standard with node auto-provisioning).
4. EKS Auto Mode.

## Decision outcome

Chosen option: "EKS + Karpenter", because Karpenter provisions the exact GPU instance a
pending pod needs (per-pool limits, spot/on-demand, Capacity Block reservations), consolidates
empty nodes, and the `terraform-aws-modules/eks` v21 karpenter submodule provides the IAM,
interruption queue and Pod Identity wiring. GKE stays a v2 extension (§A.2). EKS Auto Mode
hides the Karpenter configuration that experiments E4/E6/E7 need to control.

Topology: one VPC (3 AZs, private node subnets, public load-balancer subnets, one NAT
gateway), EKS 1.35, a small on-demand system node group (`m7i.xlarge` × 2), Pod Identity for
components that need AWS APIs, an SSE-KMS weights bucket, everything tagged
`project/run-id/owner/profile`.

### Consequences

- Good: GPU nodes exist only while inference pods need them; cost is attributable by tags.
- Good: identical module structure for aws-dev and aws-bench (inputs differ).
- Bad: single NAT gateway is a single point of failure (production HA is out of scope).
- Bad: public API endpoint (restricted to the operator's /32) — accepted risk recorded in
  `.trivyignore.yaml`, to revisit with a VPN/bastion.

### Confirmation

CI job `terraform` (fmt, validate, mocked `tofu test`, tflint AWS ruleset, Trivy config);
human run per `docs/runbooks/aws-first-run.md` (target ≤ 20 min to a reachable cluster);
GPU pools in feature 006.

## Updates

- Feature 006: GPU pools use Bottlerocket NVIDIA (`bottlerocket@v1.68.1`, driver and device
  plugin in the AMI, no GPU Operator), `WhenEmpty` consolidation after 10 minutes, RAID0
  instance store for the weight cache, and reserved capacity (Capacity Blocks) for H100.

## Links

- [specs/004-aws-infrastructure/research.md](../../specs/004-aws-infrastructure/research.md)
- <https://github.com/terraform-aws-modules/terraform-aws-eks/tree/v21.29.0>
