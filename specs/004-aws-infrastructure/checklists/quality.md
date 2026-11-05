# Requirements Quality Checklist: 004 aws-infrastructure

**Purpose**: Unit tests for the requirements — completeness, testability, security and cost-safety.

**Created**: 2026-11-05

**Feature**: [spec.md](../spec.md) · [plan.md](../plan.md)

## Completeness

- [x] CHK001 Are all created resource groups enumerated (VPC, EKS, node group, Pod Identity, bucket)? [Completeness, Spec §FR-001]
- [x] CHK002 Are all outputs the GitOps layer needs listed? [Completeness, data-model.md]
- [x] CHK003 Is the state bootstrap specified with its protections? [Completeness, Spec §FR-006]
- [x] CHK004 Is the behavior on partial apply failure defined? [Edge Cases]

## Clarity and testability

- [x] CHK005 Are the four tags named with their sources (run ID, owner)? [Clarity, Clarifications]
- [x] CHK006 Are input validations specified per variable? [Measurability, data-model.md]
- [x] CHK007 Is the CLI command sequence specified precisely enough to unit-test? [Clarity, contracts/terraform-io.md]

## Security

- [x] CHK008 Is the EKS endpoint exposure bounded (no wildcard CIDR)? [Security, Clarifications]
- [x] CHK009 Are bucket encryption, versioning, TLS-only and public-access blocks required? [Security, Clarifications, FR-001]
- [x] CHK010 Is IAM scoping per component specified? [Security, FR-008, Clarifications]
- [x] CHK011 Is it explicit that account-specific values never reach git? [Security, Edge Cases]

## Cost-safety

- [x] CHK012 Is apply gated by explicit human confirmation (`--yes` or prompt)? [Cost, contracts/cli-aws.md]
- [x] CHK013 Is the agent prohibited from running apply/destroy, with offline validation instead? [Cost, Assumptions, FR-005]
- [x] CHK014 Are cost-relevant choices justified (single NAT, node size)? [Cost, Clarifications]
