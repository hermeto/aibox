# Contributing to AIBox

Thank you for your interest in AIBox! This guide explains how to set up your environment,
how features are specified with Spec Kit, and how to open a pull request that passes CI.
By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Quick start

```bash
git clone https://github.com/hermeto/aibox && cd aibox
make dev-setup     # checks prerequisites, installs pinned linters into ./bin, pre-commit hooks
make lint test     # exactly what CI's required jobs run; no cloud credentials needed
```

Prerequisites: `git`, `go` (version from `go.mod`), [`uv`](https://docs.astral.sh/uv/),
`node` (for markdownlint) and `make`. Docker is needed from the local platform feature on.
`make dev-setup` prints the install command for anything missing on macOS (Homebrew) or
Linux (apt or official installers) — Homebrew is not required. Run `make help` for every
target.

## Working on the CLI

The CLI lives in `cmd/aibox` and `internal/` (`cli`, `profile`, `doctor`, `exec`, `output`,
`awsapi`). External commands always go through `internal/exec.Runner`, so tests use
`exec.Fake` and `--dry-run` works everywhere. Golden files (help output, doctor reports) are
refreshed with `go test ./internal/cli ./internal/doctor -update` — review the diff.

## How work is organized: Spec Kit

Every feature follows Spec-Driven Development with [Spec Kit](https://github.com/github/spec-kit):

1. `/speckit-specify` → `specs/NNN-short-name/spec.md`
2. `/speckit-clarify` → `## Clarifications` in the spec
3. `/speckit-plan` → `plan.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`
4. `/speckit-checklist` → `checklists/*.md`
5. `/speckit-tasks` → `tasks.md`
6. `/speckit-analyze` → consistency report (fix CRITICAL/HIGH findings)
7. `/speckit-implement` → code, tests, docs
8. `/speckit-converge` → append any remaining gap tasks

The [constitution](.specify/memory/constitution.md) governs every plan; architectural
decisions are recorded as ADRs in [`docs/adr/`](docs/adr/README.md). The product
specification is [`docs/product/spec.md`](docs/product/spec.md).

## Branches and pull requests

- Feature branches are named `NNN-short-name` (created by Spec Kit). Iterations on a feature
  use `NNN-short-name-iterK`.
- Large efforts are delivered as **stacked** pull requests (each based on the previous one,
  labeled `stacked`); review and merge them bottom-up.
- Use the [pull request template](.github/PULL_REQUEST_TEMPLATE.md); keep PRs under ~800
  changed lines excluding generated files, lock files and Spec Kit artifacts.
- The single required check is **`ci-gate`**, which aggregates lint, tests, PR title, DCO,
  dependency review and security scans.

## Commit conventions

- [Conventional Commits](https://www.conventionalcommits.org/): `feat(cli): add profile loader`,
  `fix(gateway): …`, `docs(adr): …`. PR titles follow the same format (checked in CI).
- **DCO sign-off is required** on every commit: use `git commit -s`. The sign-off certifies
  the [Developer Certificate of Origin](https://developercertificate.org/). The CI `dco` job
  requires a `Signed-off-by:` trailer matching the commit author's email. To fix a branch:
  `git rebase --signoff <base> && git push --force-with-lease`.
- Do not edit `CHANGELOG.md`; release-please generates it from commit messages.

## Quality gates

| Check | Local command | CI job |
| --- | --- | --- |
| Go lint and format | `make lint-go`, `make fmt` | `lint` |
| GitHub Actions lint and SHA pinning | `make lint-actions lint-pins` | `lint` |
| Shell, YAML, Markdown, EditorConfig | `make lint-shell lint-yaml lint-markdown lint-editorconfig` | `lint` |
| Unit tests (race detector) | `make test` | `test` |
| Supply chain | — | `trivy`, `dependency-review`, CodeQL, OpenSSF Scorecard |

Pre-commit runs the same `make` targets: `pre-commit run --all-files`.

## Repository configuration as code

- Default-branch ruleset: [`.github/rulesets/main.json`](.github/rulesets/main.json)
- Labels: [`.github/labels.yml`](.github/labels.yml) (synced by the `labels` workflow)
- Dependency updates: [`.github/dependabot.yml`](.github/dependabot.yml)
- Release automation: [`release-please-config.json`](release-please-config.json),
  [`.goreleaser.yaml`](.goreleaser.yaml)
- One-time admin setup for maintainers: [`hack/github/setup-repo.sh`](hack/github/setup-repo.sh)
- Issue forms: [`.github/ISSUE_TEMPLATE/`](.github/ISSUE_TEMPLATE/config.yml)

## Getting help

See [SUPPORT.md](SUPPORT.md). Security issues: [SECURITY.md](SECURITY.md). Project
governance: [GOVERNANCE.md](GOVERNANCE.md) and [MAINTAINERS.md](MAINTAINERS.md).
