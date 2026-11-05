# Runbook: first AWS run (aws-dev / aws-bench)

> **Costs money.** An `aws-dev` environment costs roughly USD 0.60/hour before any GPU node
> (EKS control plane, 2 × m7i.xlarge system nodes, NAT gateway). Always finish with
> `aibox down`. Budgets, TTL teardown and the orphan checker arrive with feature 005 — until
> then, set a manual AWS Budget alert.

This runbook is for the maintainer. The automation never runs it unattended.

## 1. Prerequisites

- AWS account with admin-equivalent credentials for the first run (`aws sts get-caller-identity` works).
- Tools: `tofu` ≥ 1.10 (or `terraform` ≥ 1.10), `kubectl`, `helm`, `aws` CLI v2 (used by the
  generated kubeconfig: `aws eks get-token`). Check with `aibox doctor --profile aws-dev`.
- Choose a region that offers the profile's GPU families (`g6` for L4, `g5` for A10G, `p5` for
  H100). `us-east-1` and `us-west-2` offer all of them. `aibox doctor` fails if a type is not
  offered.

## 2. Quotas (do this first — approvals can take days)

`aibox doctor --profile aws-dev` prints the required vCPUs per quota. Request increases in
Service Quotas → Amazon EC2:

| Quota | aws-dev | aws-bench |
| --- | --- | --- |
| All G and VT Spot Instance Requests (`L-3819A6DF`) | ≥ 16 vCPU | — |
| Running On-Demand G and VT instances (`L-DB2E81BA`) | — | ≥ 192 vCPU |
| H100 (p5) | — | EC2 Capacity Block reservation (no vCPU quota) |

## 3. Kubernetes version

The profiles pin `kubernetesVersion: "1.35"`. Check what EKS supports today and bump the
profiles (and `infra/terraform/aws` tests) if a newer version is in standard support:

```bash
aws eks describe-cluster-versions --region us-east-1 \
  --query 'clusterVersions[?versionStatus==`STANDARD_SUPPORT`].clusterVersion'
```

## 4. Create the state bucket (once per account and region)

```bash
cd infra/terraform/aws/bootstrap
tofu init
tofu apply -var region=us-east-1 -var owner="$(git config user.email)"
```

This creates `aibox-tfstate-<account>-<region>` (versioned, KMS-encrypted, public access
blocked, TLS-only, protected with `prevent_destroy`). Keep this directory's local
`terraform.tfstate` safe; it only tracks the bucket.

## 5. Bring up

```bash
make build
./bin/aibox doctor --profile aws-dev
./bin/aibox up --profile aws-dev          # shows the plan, asks for confirmation
```

- The EKS public endpoint is limited to your egress IP (/32). Pass `--admin-cidr` to use
  another CIDR (never `0.0.0.0/0`).
- Generated files live in `.aibox/aws-dev/` (git-ignored): `run.json` (run ID, owner, state
  key), `terraform.tfvars.json`, `outputs.json`, `kubeconfig`.
- Target: "cluster reachable" in ≤ 20 minutes:
  `kubectl --kubeconfig .aibox/aws-dev/kubeconfig get nodes`.

If `up` fails half-way, re-run it (Terraform resumes) or run `aibox down`.

## 6. Tear down

```bash
./bin/aibox down --profile aws-dev        # asks for confirmation; destroys everything
```

Then check the console (EC2, ELB, EBS, NAT gateways, EIPs) for leftovers tagged
`project=aibox` — feature 005 automates this check.

## Troubleshooting

| Symptom | Fix |
| --- | --- |
| `init failed … state bucket` | Run step 4 in the same account/region, or set `AIBOX_TF_STATE_BUCKET`. |
| Doctor: quota too low | Request the increase (step 2) and wait for approval. |
| `kubectl` hangs / 403 | Your IP changed: re-run `aibox up --admin-cidr <new-ip>/32`. |
| `aws eks get-token` not found | Install AWS CLI v2. |
