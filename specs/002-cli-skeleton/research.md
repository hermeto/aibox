# Research: 002 cli-skeleton

Versions resolved on 2026-11-05 with `go list -m -versions` (latest non-prerelease).

| Module | Version | Use |
| --- | --- | --- |
| github.com/spf13/cobra | v1.10.2 | command tree, help, flag parsing |
| sigs.k8s.io/yaml | v1.6.0 | YAML → JSON for schema validation and typed decoding |
| github.com/santhosh-tekuri/jsonschema/v6 | v6.0.3 | JSON Schema 2020-12 validation with instance locations |
| github.com/aws/aws-sdk-go-v2 (+config, sts, servicequotas, ec2) | v1.47.6 / v1.33.14 / v1.51.7 / v1.43.9 / v1.344.0 | read-only AWS preflight checks |

## D1 — CLI framework: Cobra (ADR-0008)

- **Decision**: Cobra without Viper; configuration comes only from profiles (YAML) and flags.
- **Rationale**: de-facto standard for Kubernetes-ecosystem CLIs (kubectl, helm, kind), gives
  consistent help, completion and docs generation (`cobra/doc`, used in feature 016).
- **Alternatives**: `urfave/cli` (less common in the ecosystem), `kong` (struct tags, fewer
  docs generators), standard `flag` (no subcommand ergonomics).

## D2 — Profile validation

- **Decision**: YAML is converted to JSON (`sigs.k8s.io/yaml`) and validated against
  `schemas/profile.schema.json` (draft 2020-12, `additionalProperties: false` everywhere),
  then decoded strictly into Go structs. Errors report the JSON pointer (`/spec/clustr`)
  and the keyword message.
- **Rationale**: one schema serves the CLI, editors (YAML language server via `$schema`)
  and CI (`check-jsonschema`).

## D3 — Doctor checks and thresholds

| Check | Provider | Pass | Warn | Fail |
| --- | --- | --- | --- | --- |
| tool `docker` ≥ 24.0 | kind | found, version ≥ min | version unparsable | missing / too old |
| tool `kind` ≥ 0.27.0 | kind | " | " | " |
| tool `kubectl` ≥ 1.31.0 | kind, aws | " | " | " |
| tool `helm` ≥ 3.15.0 | kind, aws | " | " | " |
| tool `tofu` ≥ 1.8.0 or `terraform` ≥ 1.9.0 | aws | either found | — | neither |
| docker daemon reachable | kind | `docker info` succeeds | — | error |
| docker CPUs | kind | ≥ `minCPUs` | — | < `minCPUs` |
| docker memory | kind | ≥ recommended | < recommended | < minimum |
| AWS credentials | aws | resolved + `sts:GetCallerIdentity` ok | — | API error |
| AWS region | aws | profile/env region set | — | unset |
| instance types offered | aws | all offered in region | — | any missing |
| GPU vCPU quota (per family × capacity type) | aws | quota ≥ required | — | quota < required |

Without credentials all AWS checks are `skip` ("no credentials"). Tool versions are parsed
with a tolerant semver regex from `--version`/`version --client` output.

## D4 — GPU quota lookup

- **Decision**: list EC2 quotas (`servicequotas:ListServiceQuotas`, service code `ec2`) and
  match by quota **name**: "Running On-Demand G and VT instances", "All G and VT Spot
  Instance Requests", "Running On-Demand P instances", "All P Spot Instance Requests". Known
  codes (for operators): `L-DB2E81BA`, `L-3819A6DF`, `L-417A185B`, `L-7212CCBC` — verify with
  `aws service-quotas list-service-quotas --service-code ec2`.
- **Rationale**: matching by name avoids depending on memorized codes; one paginated call.
- Required vCPUs per class = `maxNodes` × vCPUs of the largest instance type
  (`ec2:DescribeInstanceTypes`). Classes with `capacityType: capacity-block` skip quotas.

## D5 — Command runner

- **Decision**: `internal/exec.Runner` interface (`Run(ctx, Cmd) (Result, error)`) with
  `Real` (os/exec), `DryRun` (logs the command, returns success) and `Fake` (scripted
  responses keyed by command line, records calls).
- **Rationale**: every later feature (kind, helm, tofu) shells out through it, so dry-run and
  tests come for free.

## D6 — Exit codes

0 ok · 1 failure · 2 not implemented · 64 usage error. Implemented with typed errors mapped
in `main` (`cli.ExitCode(err)`).

## References

- Cobra: <https://cobra.dev/>
- jsonschema v6: <https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6>
- Service Quotas `ListServiceQuotas`: <https://docs.aws.amazon.com/servicequotas/2019-06-24/apireference/API_ListServiceQuotas.html>
- EC2 instance quotas: <https://docs.aws.amazon.com/ec2/latest/instancetypes/ec2-instance-quotas.html>
- `DescribeInstanceTypeOfferings`: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypeOfferings.html>
