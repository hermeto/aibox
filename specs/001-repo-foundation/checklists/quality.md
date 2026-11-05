# Requirements Quality Checklist: 001 repo-foundation

**Purpose**: Unit tests for the requirements — completeness, testability, security and cost-safety.

**Created**: 2026-11-05

**Feature**: [spec.md](../spec.md) · [plan.md](../plan.md)

## Completeness

- [x] CHK001 Are all governance files named with their required content (CoC version, CONTRIBUTING topics)? [Completeness, Spec §FR-001]
- [x] CHK002 Are the PR template sections enumerated (playbook §5.3)? [Completeness, Spec §FR-002]
- [x] CHK003 Is every required CI job named, including the aggregating gate? [Completeness, Spec §FR-005, contracts/ci-gate.md]
- [x] CHK004 Are the release targets (OS × arch) and signing artifacts specified? [Completeness, Spec §FR-009]
- [x] CHK005 Is behavior for pull-request-only jobs on push events defined? [Gap → resolved in contracts/ci-gate.md]

## Clarity and testability

- [x] CHK006 Is "`ci-gate` fails" defined in terms of job results (only `success` passes)? [Clarity, Spec §Clarifications]
- [x] CHK007 Is the DCO rule testable (trailer + author email, merge commits excluded)? [Measurability, Spec §Clarifications]
- [x] CHK008 Are success criteria measurable (zero secrets, 100% SHA pinning, files linked)? [Measurability, Spec §SC-001…SC-005]
- [x] CHK009 Is "least privilege" made concrete (top-level `permissions: {}`, per-job grants)? [Clarity, Spec §FR-011]

## Security

- [x] CHK010 Is a private disclosure channel and response-time target required? [Security, Spec §US3, §Clarifications]
- [x] CHK011 Are supply-chain checks enumerated (CodeQL, Scorecard, Trivy, dependency review)? [Security, Spec §FR-005, §FR-007]
- [x] CHK012 Is fork safety (no secrets) an explicit requirement? [Security, Spec §FR-013]
- [x] CHK013 Is the Trivy integration's own supply-chain risk addressed? [Security, Spec §Clarifications]

## Cost-safety

- [x] CHK014 Is it explicit that no workflow in this feature touches cloud resources or needs credentials? [Cost, Spec §FR-013, plan Constraints]

## Consistency

- [x] CHK015 Do CodeQL language requirements match what can run today (Python deferred to 014)? [Consistency, Spec §FR-007, §Clarifications]
