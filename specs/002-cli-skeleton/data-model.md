# Data model: 002 cli-skeleton

## Profile (`profiles/*.yaml`, schema `schemas/profile.schema.json`)

```yaml
apiVersion: aibox.dev/v1alpha1     # const
kind: Profile                       # const
metadata:
  name: local                       # ^[a-z0-9][a-z0-9-]{1,30}$, must match file name
  description: ...
spec:
  provider: kind | aws
  cluster:
    name: aibox-local               # DNS label
    kubernetesVersion: "1.34"       # "MAJOR.MINOR"
  local:                            # required when provider=kind, forbidden otherwise
    minCPUs: 4
    minMemoryGiB: 6
    recommendedMemoryGiB: 8
  aws:                              # required when provider=aws, forbidden otherwise
    region: us-east-1
    gpuClasses:
      - name: gpu-l4                # gpu-l4 | gpu-a10g | gpu-h100
        instanceTypes: [g6.xlarge, g6.2xlarge]
        capacityType: spot          # spot | on-demand | capacity-block
        maxNodes: 2                 # 0..64
        capacityReservationId: cr-… # only with capacity-block
  guardrails:
    ttl: 4h                         # Go duration, 30m..24h
    ownerEmail: ops@example.com     # optional, required by feature 005 for budgets
```

Validation: `additionalProperties: false` at every level; provider-conditional sections via
`if/then`; budget is **not** a profile field (runtime input, feature 005).

## Check result

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | stable, e.g. `tool.docker`, `docker.memory`, `aws.quota.g-spot` |
| `name` | string | human label |
| `status` | enum | `pass`, `warn`, `fail`, `skip` |
| `message` | string | what was observed |
| `remediation` | string | set for `warn`/`fail` |
| `details` | object | optional key/values (versions, quota numbers) |

## Doctor report

`{ "profile": "local", "provider": "kind", "checks": [CheckResult…], "summary": {"pass":n,"warn":n,"fail":n,"skip":n}, "ok": bool }`
— `ok` is false iff `summary.fail > 0`. Schema: [contracts/doctor-report.schema.json](contracts/doctor-report.schema.json).
