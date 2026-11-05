# Runbook: cost guardrails (budget, TTL teardown, orphans, cost)

Every cloud run has a **budget** (AWS Budget scoped to the run's `run-id` tag, e-mail alerts
at 50/80/100%) and a **TTL** (automatic teardown in AWS, default 4 h, at most 24 h). Design:
[ADR-0010](../adr/0010-mandatory-budget-and-ttl.md).

## Once per AWS account

1. Run any `aibox up` once (or tag any resource with `run-id`), then activate the tag as a
   cost allocation tag: Billing → Cost allocation tags → `run-id` → Activate (or
   `aws ce update-cost-allocation-tags-status --cost-allocation-tags-status TagKey=run-id,Status=Active`).
   Budgets only see the tag after activation; data appears up to 24 h later.
2. Confirm the SNS subscription e-mail ("AWS Notification - Subscription Confirmation") after
   the first `up`; until confirmed, no alert is delivered.

## Before a run

```bash
./bin/aibox doctor --profile aws-dev        # includes a worst-case cost estimate
go run ./hack/pricing --region us-east-1    # refresh prices before a benchmark campaign
```

## Starting a run

```bash
BUDGET_USD=50 ./bin/aibox up --profile aws-dev --notify-email you@example.com --ttl 4h
```

`up` refuses to start without a budget (`BUDGET_USD` or `--budget`) and an alert e-mail
(`--notify-email` or the profile's `spec.guardrails.ownerEmail`). Guardrails are applied
before the environment, so the TTL exists even if the environment apply fails.

## During a run

```bash
./bin/aibox ttl show --profile aws-dev
./bin/aibox ttl extend 2h --profile aws-dev   # never beyond 24 h after the start
./bin/aibox cost --profile aws-dev             # live estimate; Cost Explorer lags up to 24 h
```

## Ending a run

```bash
./bin/aibox down --profile aws-dev
```

`down` drains the cluster (Argo CD scaled down, Karpenter node pools and LoadBalancer
services deleted), destroys the environment, then the guardrails, then runs the orphan check.
Exit code 1 means resources tagged with the run still exist.

## When the orphan check fails

1. Read the list (type, ID). Typical leftovers: EBS volumes from PVCs, load balancers from
   Services, ENIs still detaching, Karpenter nodes.
2. Wait 5 minutes and re-check: `./bin/aibox down --profile aws-dev --orphans-only`.
3. Delete remaining resources in the console (filter by tag `run-id`), then re-check.
4. The run state in `.aibox/<profile>/` is kept until the check passes.

## When the automatic teardown fails

You receive "AIBox TTL teardown FAILED" by e-mail with the CodeBuild URL.

1. Open the CodeBuild logs (project `aibox-<run-id>-teardown`).
2. If the state lock was held (`aibox up` still running), wait and run `aibox down`.
3. Otherwise run `aibox down --profile <profile>` from a checkout of the run's commit.
4. Run the orphan check and clean up as above.

## Costs of the guardrails themselves

AWS Budgets (first two per account are free), one SNS topic, one KMS key (USD 1/month,
deleted with the run), an idle CodeBuild project (no charge until it runs; a teardown costs
cents).
