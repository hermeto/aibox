# Research: 005 cost-guardrails

## R1 — Teardown executor options

| Option | Works with laptop off | Survives cluster failure | Time limit | Verdict |
| --- | --- | --- | --- | --- |
| EventBridge Scheduler → CodeBuild running `tofu destroy` | yes | yes | 8 h | **chosen** |
| EventBridge → Lambda | yes | yes | 15 min (EKS destroy takes ~15–20 min) | rejected |
| In-cluster CronJob | yes | no (dies with the cluster / node failures) | — | rejected |
| Step Functions + SDK deletes | yes | yes | — | rejected: re-implements Terraform's dependency order |

EventBridge Scheduler supports one-time `at(yyyy-mm-ddThh:mm:ss)` schedules with
`action_after_completion = "DELETE"` and universal targets, including
`arn:aws:scheduler:::aws-sdk:codebuild:startBuild`.
Terraform: `aws_scheduler_schedule` (`schedule_expression`, `flexible_time_window`, `target`
with `arn`, `role_arn`, `input`). Source: <https://registry.terraform.io/providers/hashicorp/aws/6.72.0/docs/resources/scheduler_schedule>.

## R2 — Budgets by tag

`aws_budgets_budget` with `cost_filter { name = "TagKeyValue", values = ["user:run-id$<id>"] }`,
`budget_type = "COST"`, `time_unit = "MONTHLY"`, notifications `ACTUAL` `GREATER_THAN`
50/80/100 `PERCENTAGE` to an SNS topic (`subscriber_sns_topic_arns`) with an e-mail
subscription. The tag must be activated as a cost allocation tag (Billing console or
`aws ce update-cost-allocation-tags-status`); data appears up to 24 h later.

## R3 — CodeBuild buildspec

Image `aws/codebuild/amazonlinux-x86_64-standard:5.0`; `NO_SOURCE` with inline buildspec
(`templatefile`). Steps: download OpenTofu `tofu_<v>_linux_amd64.zip` + `tofu_<v>_SHA256SUMS`
from the GitHub release, verify the checksum, `git clone` the repository at the run commit,
write `terraform.tfvars.json` from the `AIBOX_TFVARS_B64` environment variable, `tofu init`
with the S3 backend, `tofu destroy -auto-approve` (retry 3× with backoff when the state lock is
held), then the same for the guardrails stack. `on-failure` publishes to the SNS topic.

## R4 — Prices (us-east-1, USD/hour)

Representative values; `verified: false` until `go run ./hack/pricing` refreshes them from the
Price List API (`pricing:GetProducts`, region `us-east-1`) and spot history
(`ec2:DescribeSpotPriceHistory`). Fixed items: EKS control plane 0.10, NAT gateway 0.045
(+ data processing, not modeled).

## R5 — Orphan checks (read-only)

`ec2:DescribeInstances` (`tag:run-id`, exclude `terminated`), `ec2:DescribeVolumes`,
`ec2:DescribeSnapshots` (`OwnerIds=self`), `ec2:DescribeNetworkInterfaces`,
`ec2:DescribeAddresses`, `ec2:DescribeNatGateways` (exclude `deleted`),
`elasticloadbalancing:DescribeLoadBalancers` + `DescribeTags` (v2). Recorded responses are
stored as JSON in `internal/cost/testdata/` and decoded into the SDK output structs, so the
production filtering code runs on them.
