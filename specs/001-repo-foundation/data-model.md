# Data model: 001 repo-foundation

This feature has no runtime data. The "entities" are configuration files with fixed shapes.

## Label (`.github/labels.yml`)

| Field | Type | Rule |
| --- | --- | --- |
| `name` | string | unique, lowercase, `group/value` for `kind/*` and `area/*` |
| `color` | hex string (6 chars, no `#`) | from playbook §4.4 |
| `description` | string | ≤ 100 chars |

## Required job (in `ci.yml`)

| Field | Rule |
| --- | --- |
| job id | listed in `ci-gate.needs` |
| result | one of `success`, `failure`, `cancelled`, `skipped`; only `success` passes the gate |

## ADR (`docs/adr/NNNN-title.md`)

| Field | Rule |
| --- | --- |
| number | 4 digits, sequential, never reused |
| status | `Proposed`, `Accepted`, `Deprecated`, `Superseded by ADR-NNNN`, or `Proposed — pending experiment` |
| sections | Context and problem, Decision drivers, Considered options, Decision outcome, Consequences, Links |

State transitions: `Proposed → Accepted → (Deprecated | Superseded)`.
