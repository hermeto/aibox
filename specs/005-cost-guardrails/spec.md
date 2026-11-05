# Feature Specification: Cost guardrails — budgets, TTL auto-teardown, orphan detection and `aibox cost`

**Feature Branch**: `005-cost-guardrails`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "Cost guardrails so that no AIBox cloud run can silently keep spending money: mandatory budget and TTL for cloud runs, a run-scoped AWS Budget with alerts, a TTL teardown that works with the operator's laptop off, an orphan check after `down`, `aibox cost` for the current run, and a cost estimate in `aibox doctor` from a versioned price table (playbook §6, feature 005)."

## Clarifications

### Session 2026-11-05

- Q: What executes the TTL teardown when the laptop is off? → A: An EventBridge Scheduler one-time schedule (`at(expiry)`, deleted after completion) starts an AWS CodeBuild project whose buildspec installs the pinned OpenTofu, clones the public repository at the run's commit, writes the run's inputs from environment variables and runs `tofu destroy` on the main stack, then on the guardrails stack. Failures publish to the run's SNS alert topic (the same e-mail as the budget). (decided by agent: simplest reliable executor; Lambda's 15-minute limit is too short for an EKS destroy; an in-cluster CronJob would die with the cluster — rationale in ADR-0010)
- Q: Where do guardrail resources live? → A: A separate Terraform stack `infra/terraform/aws/guardrails` with its own state key (`<profile>/<cluster>-guardrails.tfstate`), applied before the main stack and destroyed after it. (decided by agent: the TTL exists even if the main apply fails half-way, and the teardown never destroys itself mid-run)
- Q: How is the budget scoped to one run? → A: `aws_budgets_budget` with a `TagKeyValue` filter `user:run-id$<run-id>` and three ACTUAL-cost notifications (50/80/100%) to the alert e-mail via SNS. The `run-id` tag must be activated once as a cost allocation tag (runbook). (decided by agent: AWS Budgets' supported way to scope by tag)
- Q: Where does the alert e-mail come from? → A: `--notify-email`, else the profile's `guardrails.ownerEmail`; otherwise `up` stops. (decided by agent: alerts must reach a person)
- Q: How does `aibox ttl extend` move the TTL? → A: It re-applies the guardrails stack with the new `ttl_expires_at` and updates `.aibox/<profile>/run.json`; total lifetime ≤ 24 h after the run start. (decided by agent: one code path, state stays in Terraform)
- Q: How is the price table populated without Price List API access from the build environment? → A: `data/pricing/aws-us-east-1.yaml` ships representative prices with `verified: false`, its source and date; `go run ./hack/pricing` refreshes on-demand prices from the AWS Price List API and spot prices from the EC2 spot price history and sets `verified: true`. Doctor warns while the table is unverified or older than 90 days. (decided by agent: honest about provenance; the maintainer refreshes before a campaign)
- Q: How does `aibox cost` measure usage? → A: EC2 instances tagged with the run ID (running or stopped) × hours since launch × price (spot vs on-demand from the instance lifecycle), plus fixed hourly items (EKS control plane, NAT gateway) since the run start; it is an estimate and says so. (decided by agent: live, read-only, no Cost Explorer lag)
- Q: How are orphans detected? → A: Per-service read-only describe calls filtered by `tag:run-id` (EC2 instances not terminated, EBS volumes, snapshots owned by the account, ENIs, Elastic IPs, NAT gateways not deleted) and Elastic Load Balancing v2 load balancers whose tags carry the run ID (Kubernetes adds the cluster tag; the AWS Load Balancer Controller propagates `run-id` via default tags in feature 009). Fixtures are recorded API responses in `testdata/`. (decided by agent: tag-API results can lag; describe calls show the current state)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - No cloud run without a budget and a TTL (Priority: P1)

`aibox up --profile aws-*` refuses to start unless a budget is given (`BUDGET_USD` or
`--budget`). Every run has a TTL (default 4 h from the profile). The run creates an AWS
Budget scoped to the run's tag with alerts at 50%, 80% and 100% to the operator's email.

**Why this priority**: GPU clusters cost tens of dollars per hour; the project promises
"nothing left billing".

**Independent Test**: unit tests show that `up` without a budget exits before any
infrastructure command, and that the rendered guardrail inputs contain the budget, the
alert thresholds, the e-mail and the TTL expiry.

**Acceptance Scenarios**:

1. **Given** no `BUDGET_USD` and no `--budget`, **When** `aibox up --profile aws-dev` runs,
   **Then** it exits 1 explaining how to set a budget, and nothing is planned or applied.
2. **Given** a budget and an e-mail, **When** `up` runs, **Then** a budget filtered by the
   run's tag with notifications at 50/80/100% is part of the applied guardrails.
3. **Given** no e-mail in the profile or flags, **When** `up` runs, **Then** it exits 1 asking
   for `--notify-email` (alerts must reach a person).

---

### User Story 2 - The TTL tears everything down, even if the laptop is off (Priority: P1)

When the TTL expires, a teardown runs inside AWS and destroys the run's infrastructure, unless
the operator extends it with `aibox ttl extend 2h`.

**Why this priority**: forgotten clusters are the biggest cost risk.

**Independent Test**: unit tests render the teardown schedule at `start + TTL`, `ttl extend`
moves it, and the teardown job definition destroys the main stack with the run's inputs.

**Acceptance Scenarios**:

1. **Given** a run started at T with TTL 4 h, **When** guardrails are rendered, **Then** the
   one-time teardown is scheduled at T + 4 h.
2. **Given** a running environment, **When** the operator runs `aibox ttl extend 2h`,
   **Then** the schedule moves 2 h later and the new expiry is printed; extensions beyond
   24 h after the start are refused.
3. **Given** the teardown fires, **When** it runs, **Then** it destroys the main stack using
   the run's saved inputs and state, and reports failures through the budget e-mail topic.

---

### User Story 3 - `down` proves nothing is left (Priority: P1)

`aibox down` finishes with an orphan check that lists every resource still tagged with the
run ID (EC2 instances, EBS volumes and snapshots, network interfaces, load balancers, Elastic
IPs, NAT gateways) and exits non-zero if any remain.

**Why this priority**: Terraform destroy does not see resources created by controllers
(Karpenter nodes, load balancers from Services, volumes from PVCs).

**Independent Test**: unit tests run the checker against recorded API responses: a clean
account passes; leftovers are listed by type and ID and cause exit 1.

**Acceptance Scenarios**:

1. **Given** a successful destroy and no tagged leftovers, **When** `down` finishes, **Then**
   it reports "0 orphans" and exits 0.
2. **Given** a leftover volume and load balancer tagged with the run ID, **When** `down`
   finishes, **Then** both are listed with type, ID and region and the exit code is 1.
3. **Given** `aibox down --orphans-only`, **When** run, **Then** only the check runs.

---

### User Story 4 - Know what the current run costs (Priority: P1)

`aibox cost` shows the accumulated cost of the current run: instance-hours × the on-demand or
spot price of each instance type, plus fixed hourly items (EKS control plane, NAT gateway),
with a note that AWS Cost Explorer data lags by up to 24 hours.

**Why this priority**: operators need a live number during benchmark campaigns.

**Independent Test**: unit tests compute cost from recorded instance lists and the price
table, with text and JSON output.

**Acceptance Scenarios**:

1. **Given** two running instances started 90 minutes ago, **When** `aibox cost` runs,
   **Then** it prints each line item (type, capacity type, hours, price/hour, cost) and the
   total, plus the lag note.

---

### User Story 5 - Estimate before running (Priority: P2)

`aibox doctor` estimates the cost of a planned run from the profile (maximum nodes per GPU
class, system nodes, fixed items) × TTL, using a versioned price table
(`data/pricing/aws-<region>.yaml`) that records its source and "last updated" date.

**Why this priority**: helps choose TTL and budget, but does not block.

**Independent Test**: doctor's JSON includes an estimate check with the worst-case cost and
the price table date; a stale table (> 90 days) produces a warning.

**Acceptance Scenarios**:

1. **Given** the aws-dev profile and a 4 h TTL, **When** doctor runs, **Then** it reports the
   worst-case cost of the run and warns if the budget is below it.

### Edge Cases

- Teardown while Terraform state is locked → the teardown retries (bounded) and then alerts.
- Partial apply failure → guardrails are applied **before** the main stack, so the TTL and
  budget exist even if the main apply fails half-way.
- Spot instances already reclaimed → they are no longer billed; cost counts only reported
  running time (documented approximation).
- Price table missing a type → the cost line is marked "price unknown" and the total is a
  lower bound.
- `ttl extend` without a running environment → exits 1 with "no active run".

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `up` for cloud profiles MUST require a budget (`BUDGET_USD` or `--budget`, > 0)
  and an alert e-mail, and MUST use the profile TTL unless `--ttl` overrides it (30 m–24 h).
- **FR-002**: Guardrails MUST be created before the main infrastructure and destroyed after
  it: a run-scoped AWS Budget (filter by run tag) with alerts at 50/80/100%, and a one-time
  teardown schedule at expiry.
- **FR-003**: The teardown MUST run inside AWS without the operator's machine and destroy the
  main stack with the run's saved inputs and state.
- **FR-004**: `aibox ttl extend <duration>` MUST move the teardown time; the total lifetime
  MUST NOT exceed 24 h from the run start.
- **FR-005**: `down` MUST finish with a read-only orphan check over EC2 instances, EBS
  volumes, snapshots, network interfaces, load balancers, Elastic IPs and NAT gateways tagged
  with the run ID, exiting non-zero if any remain.
- **FR-006**: `aibox cost` MUST report the current run's accumulated cost from instance-hours ×
  price table plus fixed hourly items, in text and JSON, with a lag note for Cost Explorer.
- **FR-007**: A versioned price table per region MUST record its source URL and last-updated
  date; a script (not run in CI) MUST be able to refresh it.
- **FR-008**: `aibox doctor` MUST report a worst-case cost estimate for the planned run and
  warn when the budget is lower or the price table is older than 90 days.
- **FR-009**: All AWS calls of the orphan checker and `cost` MUST be read-only.

### Key Entities

- **Guardrails**: budget amount, alert thresholds, e-mail, TTL start/expiry, teardown target.
- **Price table**: region, currency, source, last-updated date, on-demand and spot prices per
  instance type, fixed hourly items.
- **Orphan**: resource type, ID, region, tags.
- **Cost report**: run ID, window, line items, total, notes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of cloud `up` invocations without a budget stop before any infrastructure
  command (unit-tested).
- **SC-002**: The teardown schedule is always `start + TTL` (± 1 minute) and never more than
  24 h after the start (unit-tested).
- **SC-003**: The orphan checker detects every resource type in FR-005 from recorded fixtures.
- **SC-004**: Runbook `docs/runbooks/cost-guardrails.md` explains budgets, TTL, extension,
  orphan cleanup and what to do when a teardown fails.

## Assumptions

- AWS Budgets can filter by a user-defined cost allocation tag; the `run-id` tag must be
  activated as a cost allocation tag once per account (runbook step).
- Spot prices change; the price table stores a representative recent value per type.
- The agent never applies guardrails or runs teardowns; tests use fakes and mocked providers.
