# Data model: 004 aws-infrastructure

## Inputs (`.aibox/<profile>/terraform.tfvars.json`, rendered by the CLI)

| Variable | Type | Validation |
| --- | --- | --- |
| `profile` | string | `^aws-[a-z0-9-]+$` |
| `run_id` | string | `^[a-z0-9-]{8,40}$` |
| `owner` | string | non-empty, ≤ 128 chars |
| `region` | string | AWS region pattern |
| `cluster_name` | string | DNS label ≤ 40 |
| `kubernetes_version` | string | `^1\.[0-9]{2}$` |
| `admin_cidrs` | list(string) | ≥ 1, each a valid CIDR, no `0.0.0.0/0` |
| `az_count` | number | 2–3 (default 3) |
| `vpc_cidr` | string | valid CIDR (default `10.42.0.0/16`) |
| `system_node_instance_types` | list(string) | default `["m7i.xlarge"]` |
| `system_node_min/desired/max` | number | 1 ≤ min ≤ desired ≤ max ≤ 10 |

## Outputs (`.aibox/<profile>/outputs.json` = `tofu output -json`)

`cluster_name`, `cluster_endpoint`, `cluster_certificate_authority_data`, `oidc_issuer_url`,
`region`, `vpc_id`, `private_subnet_ids`, `weights_bucket`, `weights_kms_key_arn`,
`karpenter_queue_name`, `karpenter_node_role_name`, `role_arns` (map: external-secrets,
otel-collector, model-sync, model-server), `tags`.

## Run state (`.aibox/<profile>/run.json`)

`{ "runId": "...", "owner": "...", "createdAt": "RFC3339", "region": "...", "stateBucket": "...", "stateKey": "..." }`
— created on first `up`, deleted after a successful `down`.
