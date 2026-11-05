# Research: 004 aws-infrastructure

Resolved 2026-11-05 from upstream git tags (`git ls-remote`) and the module sources.

| Component | Version | Notes |
| --- | --- | --- |
| OpenTofu | 1.13.8 | CI engine (`opentofu/setup-opentofu`) |
| Terraform | 1.17.1 | compatible; used for local checks |
| hashicorp/aws provider | 6.72.0 | EKS module needs ≥ 6.59 |
| terraform-aws-modules/vpc | v6.7.3 | needs aws ≥ 6.28 |
| terraform-aws-modules/eks | v21.29.0 | needs aws ≥ 6.59, tls ≥ 4.0, time ≥ 0.9; Terraform ≥ 1.5.7 |
| eks `modules/karpenter` | v21.29.0 | controller role (Pod Identity), node role, SQS interruption queue + EventBridge rules |
| tflint / ruleset-aws | v0.65.0 / v0.50.0 | |
| EKS Kubernetes | 1.35 | newest in eksctl v0.231.0 `SupportedVersions`; AWS docs unreachable from the build environment — operator confirms with `aws eks describe-cluster-versions` |

## R1 — Module sources

Git sources pinned to tags (`git::https://github.com/terraform-aws-modules/terraform-aws-eks.git?ref=v21.29.0`)
instead of registry addresses: identical content, installable where the registry is not
reachable, and Dependabot's terraform ecosystem updates tag refs.

## R2 — EKS v21 inputs used

`name`, `kubernetes_version`, `vpc_id`, `subnet_ids`, `control_plane_subnet_ids`,
`endpoint_public_access`, `endpoint_public_access_cidrs`, `endpoint_private_access`,
`enable_cluster_creator_admin_permissions`, `enabled_log_types`, `addons` (coredns,
kube-proxy, vpc-cni with `before_compute`, eks-pod-identity-agent), `eks_managed_node_groups`
(`system`: `instance_types`, `capacity_type`, `min_size`, `max_size`, `desired_size`,
`labels`, `metadata_options` with IMDSv2), `node_security_group_tags` (Karpenter discovery).
Checked in `variables.tf` of the module at v21.29.0 (v21 renamed `cluster_name` → `name`,
`cluster_version` → `kubernetes_version`, `cluster_addons` → `addons`).

## R3 — Karpenter submodule

`modules/karpenter` inputs: `cluster_name`, `create_pod_identity_association = true`,
`namespace = "karpenter"`, `service_account = "karpenter"`, `node_iam_role_use_name_prefix`,
`node_iam_role_additional_policies` (SSM). Outputs `queue_name`, `node_iam_role_name`,
`iam_role_arn` are passed to feature 006.

## R4 — Pod Identity

`aws_eks_pod_identity_association` (cluster, namespace, service account, role ARN) and roles
trusting `pods.eks.amazonaws.com` with `sts:AssumeRole` + `sts:TagSession`. Requires the
`eks-pod-identity-agent` addon.

## R5 — State backend

S3 backend with `use_lockfile = true` (native locking; Terraform ≥ 1.10, OpenTofu ≥ 1.10).
Partial backend config in `backend.tf`; the CLI passes `bucket`, `key`, `region`.

## R6 — Testing with mocks

`mock_provider "aws" {}` + `override_module` for the VPC/EKS/Karpenter modules keeps tests
offline and fast; AIBox modules are tested directly with `module { source = ... }` run
blocks and `mock_data` for `aws_iam_policy_document`/`aws_caller_identity`.

## Decisions

| ID | Decision | Rejected |
| --- | --- | --- |
| D1 | Single root + rendered tfvars | root per profile (duplication) |
| D2 | One NAT gateway | NAT per AZ (cost; HA out of scope) |
| D3 | Endpoint public, CIDR = operator /32 | private only (needs VPN/bastion) |
| D4 | SSE-KMS CMK + bucket key | SSE-S3 (Trivy HIGH "customer key") |
| D5 | No KEDA AWS identity | empty role (noise; least privilege) |

## References

- <https://github.com/terraform-aws-modules/terraform-aws-eks/tree/v21.29.0>
- <https://github.com/terraform-aws-modules/terraform-aws-vpc/tree/v6.7.3>
- <https://developer.hashicorp.com/terraform/language/tests/mocking>
- <https://opentofu.org/docs/cli/commands/test/>
- <https://developer.hashicorp.com/terraform/language/backend/s3>
- <https://github.com/eksctl-io/eksctl/blob/v0.231.0/pkg/apis/eksctl.io/v1alpha5/types.go>
