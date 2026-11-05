# ADR-0010: Mandatory budget and TTL for cloud runs

- **Status**: Accepted
- **Date**: 2026-11-05
- **Deciders**: maintainer (guardrails mandated by product spec §A.5); executor chosen by the agent on the maintainer's behalf
- **Decided by**: feature 005

## Context and problem statement

A forgotten GPU cluster costs hundreds of dollars a day. AIBox promises "one command up, one
command down, nothing left billing" (constitution principle IV). The teardown must happen
even when the operator's laptop is off, and nothing must be left behind by controllers that
Terraform does not track (Karpenter nodes, load balancers, volumes).

## Decision drivers

- Works with the laptop off and with the cluster broken.
- Long enough for an EKS destroy (15–20 minutes, more with retries).
- No custom images or code to maintain; reuses the same Terraform the operator runs.
- Alerts reach a person.

## Considered options

1. EventBridge Scheduler (one-time) → CodeBuild running `tofu destroy`.
2. EventBridge → Lambda calling AWS APIs.
3. In-cluster CronJob running the teardown.
4. Step Functions orchestrating SDK deletes.

## Decision outcome

Chosen option: "EventBridge Scheduler → CodeBuild", because CodeBuild runs for up to 8 hours
(Lambda stops at 15 minutes), lives outside the cluster (a CronJob dies with it), and runs
the exact same `tofu destroy` as `aibox down` (Step Functions would re-implement Terraform's
dependency order).

The guardrails live in a separate stack (`infra/terraform/aws/guardrails`, own state key),
applied **before** the main stack and destroyed **after** it: an AWS Budget filtered by the
`run-id` tag with alerts at 50/80/100% to an SNS topic with the operator's e-mail, the
CodeBuild project, and a one-time schedule at `start + TTL` (default 4 h, at most 24 h,
extendable with `aibox ttl extend`). `aibox up` refuses to start without a budget and an
alert e-mail. `aibox down` (and the teardown job) first drain the cluster (Argo CD scaled
down, Karpenter node pools and LoadBalancer services deleted), then destroy, then the CLI runs
a read-only orphan check by tag.

### Consequences

- Good: a run cannot outlive its TTL by more than the teardown duration; costs are bounded by
  the budget alerts and the TTL.
- Good: the orphan checker catches what Terraform cannot see.
- Bad: the teardown role needs broad delete permissions (`PowerUserAccess` + `IAMFullAccess`).
  It is assumable only by CodeBuild in the same account (`aws:SourceAccount` condition) and is
  deleted with the run. Accepted risk.
- Bad: the teardown clones the repository from a public URL; private forks need a token
  (out of scope, documented).
- Bad: AWS Budgets need the `run-id` cost allocation tag activated once per account, and cost
  data lags up to 24 h — the TTL, not the budget, is the hard stop.

### Confirmation

Unit tests: `up` refuses without budget/e-mail before any command; expiry = start + TTL;
extension ≤ 24 h; guardrails applied before and destroyed after the main stack; orphan
detection over recorded API responses; mocked `tofu test` of the guardrails stack (budget
filter, 50/80/100% alerts, one-time schedule). Human verification: runbook
`docs/runbooks/cost-guardrails.md`.

## Links

- [specs/005-cost-guardrails/research.md](../../specs/005-cost-guardrails/research.md)
- EventBridge Scheduler one-time schedules: <https://docs.aws.amazon.com/scheduler/latest/UserGuide/schedule-types.html>
