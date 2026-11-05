# Requirements Quality Checklist: 005 cost-guardrails

**Purpose**: Unit tests for the requirements — completeness, testability, security and cost-safety.

**Created**: 2026-11-05

**Feature**: [spec.md](../spec.md) · [plan.md](../plan.md)

- [x] CHK001 Is it specified what happens without a budget or e-mail (exit before any infra command)? [Completeness, US1]
- [x] CHK002 Is the TTL range and extension limit quantified (30 m–24 h; ≤ start + 24 h)? [Clarity, FR-001, FR-004]
- [x] CHK003 Is the teardown independent of the operator's machine and of the cluster? [Cost, Clarifications, R1]
- [x] CHK004 Are guardrails ordered before/after the main stack? [Completeness, FR-002]
- [x] CHK005 Are all orphan resource types enumerated? [Completeness, FR-005]
- [x] CHK006 Is the cost formula and its approximation stated? [Clarity, Clarifications]
- [x] CHK007 Is price provenance (source, date, verified flag) required? [Measurability, FR-007]
- [x] CHK008 Are orphan and cost calls read-only? [Security, FR-009]
- [x] CHK009 Is the teardown role's breadth acknowledged and bounded? [Security, plan Complexity Tracking]
- [x] CHK010 Is the cost-allocation-tag activation prerequisite documented? [Completeness, Assumptions, R2]
- [x] CHK011 Are lock contention and partial failures covered? [Edge Cases]
