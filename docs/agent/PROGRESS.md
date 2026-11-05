# Agent progress — AIBox

> A fresh session resumes from this file alone. Read `docs/agent/PLAYBOOK.md` for the
> procedure and `CLAUDE.md` for the rules.

## Current state

| Field | Value |
| --- | --- |
| Current feature | 004-aws-infrastructure |
| Spec Kit phase | implement + converge done; waiting for CI on PR #6 |
| Last commit | see `git log -1` on `004-aws-infrastructure` |
| Run status | in progress |

## Open PRs (merge order, bottom-up)

1. [#1](https://github.com/hermeto/aibox/pull/1) `001-repo-foundation` → `claude/jolly-albattani-wcnudk`
2. [#4](https://github.com/hermeto/aibox/pull/4) `002-cli-skeleton` → `001-repo-foundation`
3. [#5](https://github.com/hermeto/aibox/pull/5) `003-local-platform` → `002-cli-skeleton`
4. [#6](https://github.com/hermeto/aibox/pull/6) `004-aws-infrastructure` → `003-local-platform`

## Blockers / `needs-human`

- [#2](https://github.com/hermeto/aibox/issues/2) one-time repository setup (default branch rename, settings, ruleset, labels, milestones).

Tracking issue: [#3 v1.0 roadmap](https://github.com/hermeto/aibox/issues/3).

## Environment (prerequisites check, playbook §3)

The run executes in a managed Linux cloud container (Claude Code on the web), not on the
maintainer's Mac. Adaptations are listed under "Run decisions".

| Tool | Version | How installed |
| --- | --- | --- |
| git | 2.x (container) | preinstalled |
| Go | 1.27.3 | `GOTOOLCHAIN=go1.27.3` (latest stable on proxy.golang.org) |
| uv / Python | uv 0.11.32 / Python 3.13 | preinstalled |
| Spec Kit CLI (`specify`) | 1.3.0 (latest tag) | `uv tool install specify-cli --from git+https://github.com/github/spec-kit.git@v1.3.0` |
| Docker | 29.9.2 | preinstalled; daemon started manually |
| kind | v0.33.0 | `go install` |
| kustomize | v5.8.3 | `go install` |
| helm | v3 latest (`go install`) | `go install` |
| golangci-lint | v2.15.0 | `go install` |
| actionlint | v1.7.12 | `go install` |
| editorconfig-checker | v3.11.3 | `make tools` |
| shellcheck / yamllint / markdownlint-cli2 / pre-commit | 0.11.0 / 1.38.0 / 0.24.0 / 4.7.0 | `make tools`, `uvx`, `npx` |
| GoReleaser | v2.20.0 | `go run` (`make release-check`) |
| kubeconform, tflint, tofu, promtool, kubectl | installed when first needed | `go install` / dl.k8s.io |
| GitHub access | `hermeto` via GitHub MCP tools | `gh` CLI is not usable for GitHub operations in this environment |

## Run decisions (taken by the agent on the maintainer's behalf)

1. **Runtime environment.** No Homebrew/macOS: tools are installed with `go install`,
   `uv tool install`, `npm` and official download hosts. Some hosts are blocked from the
   container (GitHub release downloads, quay.io, registry.k8s.io, registry.terraform.io,
   Helm chart repos), so full `e2e-local` and `tofu init` with providers are verified in
   GitHub Actions CI rather than locally.
2. **GitHub CLI.** `gh` is not authenticated for repository operations here; GitHub
   operations go through the GitHub MCP tools (PRs, issues, CI status, logs). The hard stop
   "gh not authenticated" is considered not to apply because equivalent authenticated
   access exists.
3. **Repository already exists.** `hermeto/aibox` was pre-created (empty), so `gh repo
   create` is skipped. Repository settings that the available tools cannot change (topics,
   description, merge settings, security features, ruleset, label colors, milestones,
   pinned issue, Discussions) are captured as code in feature 001
   (`hack/github/setup-repo.sh`, `.github/rulesets/main.json`, `.github/labels.yml`) and in
   a `needs-human` issue for the maintainer to run once.
4. **Branches.** A direct push to `main` was refused by this session's permission policy,
   which only allows pushes to the session branch `claude/jolly-albattani-wcnudk`. The
   bootstrap commit therefore lives on `claude/jolly-albattani-wcnudk`, which acts as the
   playbook's `main` (it is the repository's first and default branch). Feature 001's PR
   targets it. **Maintainer action:** rename `claude/jolly-albattani-wcnudk` to `main` in
   GitHub (Settings → Branches → rename); GitHub retargets open PRs automatically. The
   playbook's stacked feature branch names (`001-repo-foundation` …) are used for features
   because the user asked for one PR per feature.
5. **Commit identity.** Commits are authored by the agent identity configured in the
   container and signed off (`git commit -s`) for DCO.
6. **AWS credentials in the container.** The container exposes `AWS_*` environment
   credentials that STS rejects as invalid. Nothing mutating is ever run; the agent does not
   run AWS-profile doctor checks locally (CI has no credentials, so they skip there).
7. **Dependency review.** The job first checks that the dependency graph is enabled and
   warns (instead of failing) while it is disabled; it enforces automatically once enabled
   (see #2).
8. **Image digests from CI.** Registries whose blobs are unreachable from the agent's
   container (quay.io, ghcr blobs) are pinned with digests read from the `e2e-local` job's
   "Running images (digests)" step.
9. **Terraform locally.** registry.terraform.io / registry.opentofu.org are unreachable from
   the container: providers come from releases.hashicorp.com into a filesystem mirror and
   modules through a throwaway local registry shim (scratchpad only, never committed). CI
   uses the real registries.
