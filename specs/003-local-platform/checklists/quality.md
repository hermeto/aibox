# Requirements Quality Checklist: 003 local-platform

**Purpose**: Unit tests for the requirements — completeness, testability, security and cost-safety.

**Created**: 2026-11-05

**Feature**: [spec.md](../spec.md) · [plan.md](../plan.md)

## Completeness

- [x] CHK001 Are all bring-up steps and their order specified? [Completeness, contracts/up-down.md]
- [x] CHK002 Is the failure output for a stuck Application specified? [Completeness, Spec §US1 AC4, FR-006]
- [x] CHK003 Are endpoints and host ports specified? [Completeness, Clarifications, data-model.md]
- [x] CHK004 Is the smoke report shape specified? [Completeness, contracts/smoke-report.md]
- [x] CHK005 Is the behavior for unpushed commits defined? [Completeness, Spec §US4 AC2]

## Clarity and testability

- [x] CHK006 Is "ready" defined precisely (Synced + Healthy; Deployment availability)? [Clarity, research R4]
- [x] CHK007 Is the 10-minute target tied to a runner size? [Measurability, Spec §SC-002]
- [x] CHK008 Is idempotency testable (second `up` in CI)? [Measurability, Spec §SC-003]
- [x] CHK009 Are the required metric names enumerated? [Clarity, research R2]

## Security

- [x] CHK010 Are host ports bound to loopback only? [Security, Clarifications, research R1]
- [x] CHK011 Is the Argo CD admin password kept out of output? [Security, plan Constitution Check VIII]
- [x] CHK012 Are images pinned by digest (or a documented pinning step)? [Security, research R2, R3]

## Cost-safety

- [x] CHK013 Is it explicit that no cloud resources or credentials are involved? [Cost, plan Constraints]

## Consistency

- [x] CHK014 Does the port plan leave room for the gateway (009) and Grafana (010)? [Consistency, Assumptions]
