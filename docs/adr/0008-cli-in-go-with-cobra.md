# ADR-0008: CLI in Go with Cobra

- **Status**: Accepted
- **Date**: 2026-11-05
- **Deciders**: maintainer (language fixed in product spec §A.9); framework chosen by the agent on the maintainer's behalf
- **Decided by**: feature 002

## Context and problem statement

Every AIBox operation (bring-up, teardown, model sync, smoke tests, benchmarks, reports,
cost) goes through one CLI so the same logic runs locally, in CI and against the cloud. The
product spec fixes the language (Go, single binary via GoReleaser). This ADR records the
command framework and configuration approach.

## Decision drivers

- One static binary for macOS/Linux × amd64/arm64.
- Consistent help, flags and generated reference docs (feature 016).
- Native access to client-go for Kubernetes operations (feature 003+).
- Minimal configuration magic: profiles are explicit YAML files validated by a JSON Schema.

## Considered options

1. Cobra, no Viper (profiles + flags only).
2. Cobra + Viper (env/config file layering).
3. `urfave/cli` v3.
4. `kong`.

## Decision outcome

Chosen option: "Cobra, no Viper", because Cobra is the de-facto standard in the Kubernetes
ecosystem (kubectl, helm, kind), generates Markdown reference docs (`cobra/doc`) and shell
completion, and the explicit profile file makes Viper's layered configuration unnecessary
(constitution principle IX).

### Consequences

- Good: familiar UX for Kubernetes users; docs generated from the command tree.
- Good: configuration has a single source (profile) plus flags — easy to record in benchmark
  metadata.
- Bad: environment-variable overrides must be added explicitly where needed (e.g.
  `BUDGET_USD` in feature 005).

### Confirmation

Golden tests on `aibox --help` (`internal/cli/testdata/help.golden`); CI job `cli` builds all
four release targets and runs `aibox doctor --profile local`.

## Links

- [specs/002-cli-skeleton/research.md](../../specs/002-cli-skeleton/research.md)
- Cobra: <https://cobra.dev/>
