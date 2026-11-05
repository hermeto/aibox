# Data model: 005 cost-guardrails

## Price table (`data/pricing/aws-<region>.yaml`)

```yaml
region: us-east-1
currency: USD
source: <URL or tool>
lastUpdated: "2026-11-05"
verified: false
instances:
  g6.xlarge: { onDemand: 0.8048, spot: 0.3500 }
fixed:
  eks-control-plane: 0.10
  nat-gateway: 0.045
```

## Run state additions (`run.json`)

`budgetUSD`, `notifyEmail`, `ttl`, `startedAt`, `expiresAt`, `repoURL`, `revision`.

## Guardrails stack inputs

`profile`, `run_id`, `owner`, `region`, `cluster_name`, `budget_usd` (> 0), `notify_email`,
`ttl_expires_at` (RFC3339, ≤ start + 24 h), `main_tfvars_json`, `state_bucket`,
`main_state_key`, `guardrails_state_key`, `repo_url`, `revision`, `tofu_version`.

## Orphan

`{ type: instance|volume|snapshot|eni|eip|nat-gateway|load-balancer, id, region, state }`.

## Cost report

`{ runId, region, from, to, items: [{ name, type, capacity, hours, pricePerHour, cost, priceKnown }], total, lowerBound, notes[] }`.
