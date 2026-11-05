# Implementation Plan: `aibox` CLI, profiles and preflight checks

**Branch**: `002-cli-skeleton` | **Date**: 2026-11-05 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/002-cli-skeleton/spec.md`

## Summary

Replace the placeholder with the real `aibox` CLI: a Cobra command tree with global flags,
YAML profiles validated against `schemas/profile.schema.json`, an `aibox doctor` that runs
local checks (tools + minimum versions, Docker daemon and resources) and read-only AWS
checks (credentials, region, instance-type offerings, GPU vCPU quotas) behind interfaces,
a command runner with dry-run and recorded fakes, text/JSON output, slog logging and
build-time version injection. Unimplemented commands exit 2 with a roadmap pointer.
`make up|down|models-sync|smoke|bench|report|cost` wrap the CLI. ADR-0008 records the
choice of Go + Cobra.

## Technical Context

**Language/Version**: Go 1.27.3.

**Primary Dependencies**: `github.com/spf13/cobra` v1.10.2; `sigs.k8s.io/yaml` v1.6.0;
`github.com/santhosh-tekuri/jsonschema/v6` v6.0.3; `github.com/aws/aws-sdk-go-v2` v1.47.6 with
`config` v1.33.14, `service/sts` v1.51.7, `service/servicequotas` v1.43.9, `service/ec2`
v1.344.0; standard library `log/slog`, `os/exec`, `embed`.

**Storage**: profile YAML files in `profiles/`; JSON Schema embedded in the binary from
`schemas/`.

**Testing**: `go test -race -cover`; table-driven unit tests; golden files under
`internal/cli/testdata/` and `internal/doctor/testdata/` (updated with `-update`); fakes for
the command runner, tool lookup, Docker info and AWS client.

**Target Platform**: macOS and Linux, amd64 and arm64.

**Project Type**: CLI.

**Performance Goals**: `aibox doctor --profile local` completes in < 5 s on a CI runner.

**Constraints**: no cloud credentials needed for build/lint/test; AWS calls read-only
(`sts:GetCallerIdentity`, `servicequotas:GetServiceQuota`, `ec2:DescribeInstanceTypes`,
`ec2:DescribeInstanceTypeOfferings`); stdout carries only results (JSON-clean with
`--output json`).

**Scale/Scope**: ~2,000 lines of Go including tests.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
| --- | --- | --- |
| I. Evidence over opinion | Pass | ADR-0008 (CLI in Go with Cobra) with criteria. |
| II. Reproducible | Pass | `go.mod`/`go.sum` pinned; version injected by GoReleaser. |
| III. Local-first | Pass | `doctor --profile local` runs in CI; AWS checks skip without credentials. |
| IV. Zero orphans / no unattended spend | Pass | Only read-only AWS calls; `up` for AWS not implemented here. |
| V. Declarative | Pass | Profiles are declarative YAML validated by schema. |
| VI. Test what matters | Pass | ≥ 80% coverage on `profile` and `doctor`; golden tests for help and doctor JSON. |
| VII. Observability | Pass | Structured `slog` logs on stderr; JSON output for machines. |
| VIII. Secure by default | Pass | No credentials stored; AWS SDK default chain only. |
| IX. Simplicity | Pass | No config framework (no Viper); small interfaces only where tests need them. |
| X. OSS citizenship | Pass | CONTRIBUTING updated with CLI dev loop. |

Post-design re-check: unchanged.

## Project Structure

### Documentation (this feature)

```text
specs/002-cli-skeleton/
├── plan.md  research.md  data-model.md  quickstart.md
├── contracts/{cli.md,doctor-report.schema.json}
├── checklists/{requirements.md,quality.md}
└── tasks.md
```

### Source Code (repository root)

```text
cmd/aibox/main.go                 # wires cli.Execute, maps errors to exit codes
internal/version/version.go       # Version, Commit, Date (ldflags)
internal/cli/                     # root.go, notimpl.go, doctor.go, version.go, errors.go (+ tests, testdata/*.golden)
internal/profile/                 # profile.go (types), load.go (resolve + validate) (+ tests, testdata/)
internal/doctor/                  # doctor.go (report), local.go, aws.go, tools.go (+ tests, testdata/)
internal/exec/                    # runner.go (Runner, Real, DryRun), fake.go (Fake)
internal/output/                  # output.go (text/json printers)
schemas/profile.schema.json       # JSON Schema (draft 2020-12)
schemas/embed.go                  # package schemas: go:embed *.json
profiles/{local,aws-dev,aws-bench}.yaml
docs/adr/0008-cli-in-go-with-cobra.md
Makefile                          # build + thin wrappers (up, down, models-sync, smoke, bench, report, cost)
```

**Structure Decision**: Appendix A §8 layout; `schemas/` doubles as a tiny Go package so the
schema is both a reviewable file and embedded in the binary.

## Complexity Tracking

No violations.
