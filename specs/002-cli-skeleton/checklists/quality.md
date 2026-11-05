# Requirements Quality Checklist: 002 cli-skeleton

**Purpose**: Unit tests for the requirements — completeness, testability, security and cost-safety.

**Created**: 2026-11-05

**Feature**: [spec.md](../spec.md) · [plan.md](../plan.md)

## Completeness

- [x] CHK001 Are all nine commands and five global flags enumerated? [Completeness, Spec §FR-001]
- [x] CHK002 Is the delivering feature named for every unimplemented command? [Completeness, contracts/cli.md]
- [x] CHK003 Are minimum tool versions and resource thresholds specified? [Completeness, research.md D3, Clarifications]
- [x] CHK004 Is the behavior for missing profile, unknown key and file-path profiles defined? [Completeness, Spec §US3]

## Clarity and testability

- [x] CHK005 Are exit codes defined for every outcome, including usage errors? [Clarity, Clarifications, contracts/cli.md]
- [x] CHK006 Is the doctor JSON shape specified as a schema? [Measurability, contracts/doctor-report.schema.json]
- [x] CHK007 Is the GPU quota requirement computed by a stated formula? [Clarity, Clarifications]
- [x] CHK008 Is coverage measurable per package (≥ 80% profile, doctor)? [Measurability, Spec §SC-001]

## Security

- [x] CHK009 Are AWS calls restricted to read-only APIs, enumerated? [Security, plan Constraints]
- [x] CHK010 Is "no credentials" handled without calling AWS? [Security, Spec §US4, Clarifications]
- [x] CHK011 Is it specified that credentials are never stored or printed? [Security, plan Constitution Check VIII]

## Cost-safety

- [x] CHK012 Is it explicit that `up` for AWS profiles does nothing in this feature? [Cost, contracts/cli.md]
- [x] CHK013 Does doctor fail before bring-up when the region lacks the GPU types? [Cost, Spec §US4 AC3]

## Consistency

- [x] CHK014 Is the budget kept out of the profile consistently with feature 005? [Consistency, Assumptions, data-model.md]
