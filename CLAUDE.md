# CLAUDE.md — agent rules for the AIBox repository

**Read `docs/agent/PROGRESS.md` first.** It records the current feature, the current
Spec Kit phase, the open PRs in merge order and any blockers. The full playbook is
`docs/agent/PLAYBOOK.md`; the product spec is `docs/product/spec.md`; the constitution is
`.specify/memory/constitution.md`.

## Operating rules (condensed from playbook §2)

### Autonomy and decisions

1. Never wait for the human. Answer Spec Kit questions (e.g. `/speckit-clarify`) yourself
   using the product spec, the constitution and the product goals. Prefer the option that
   keeps the `local` profile working, is simplest, and is easiest to measure.
2. Record every decision taken on the maintainer's behalf:
   - in the feature's `spec.md` under `## Clarifications`
     (`- Q: … → A: … (decided by agent: <rationale>)`);
   - as an ADR in `docs/adr/` when it is architectural;
   - in the PR description under **"Decisions for the reviewer"**.
3. Blocked? Open an issue labeled `needs-human` with the blocker and options, reference it
   in the PR, mark affected tasks `[~]` with a link in `tasks.md`, and continue.
4. Resolve the latest stable version of every tool, chart, image and module, then pin it
   (lock files, chart versions, image digests, action SHAs). Never use `latest` tags.
5. Verify upstream APIs of fast-moving projects (llm-d, Gateway API Inference Extension,
   KServe, WVA, Karpenter) against the pinned version; cite doc URLs in `research.md`.

### Hard stops — never do these

- No command that creates, changes or deletes cloud resources or costs money
  (`terraform|tofu apply|destroy`, mutating `aws` calls, `make up|down PROFILE=aws-*`,
  `eksctl create`). Validate Terraform with `fmt`, `validate`, `tflint`, `trivy config` and
  `terraform test` with mock providers only.
- No push to `main` after the bootstrap commit; no force-push to `main`. Force-push to own
  feature branches only with `--force-with-lease`.
- No merging or approving PRs, no repo visibility changes, no repo deletion, no edits to
  other repositories.
- No secrets in git (no real API keys, tokens or account IDs).
- Never disable CI checks, linters or tests to get green: fix the cause or open a
  `needs-human` issue.

### Session continuity

Keep `docs/agent/PROGRESS.md` updated after every step on every branch, and keep the
"v1.0 roadmap" tracking issue updated (checklist of PRs in merge order).

## Feature loop (playbook §5)

Branches are stacked: `main ← 001-repo-foundation ← 002-cli-skeleton ← … ← 016-docs-and-release`.
Iterations use `NNN-short-name-iterK` branched from the feature branch.

| # | Step | Commit message |
| --- | --- | --- |
| 1 | checkout base; `/speckit-specify` | `docs(spec): specify NNN <feature>` |
| 2 | `/speckit-clarify` (answer yourself) | `docs(spec): clarify NNN` |
| 3 | `/speckit-plan` (+ global plan context) | `docs(plan): plan NNN` |
| 4 | `/speckit-checklist` | `docs(spec): checklist NNN` |
| 5 | `/speckit-tasks` | `docs(tasks): tasks NNN` |
| 6 | `/speckit-analyze`; fix CRITICAL/HIGH | `docs(spec): resolve analysis findings NNN` |
| 7 | push; open draft PR | — |
| 8 | `/speckit-implement` phase by phase | Conventional Commits |
| 9 | `make lint test` + Definition of done | `fix(...)`/`test(...)` |
| 10 | `/speckit-converge` until no gaps | `fix(...)` |
| 11 | update docs/ADRs (never edit CHANGELOG) | `docs(...)` |
| 12 | push; CI green; mark ready; update roadmap + PROGRESS | — |

PRs: Conventional Commit titles ending with `(#NNN)`, body from
`.github/PULL_REQUEST_TEMPLATE.md`, labels `kind/*`, `area/*`, `spec`, `stacked`; commits
signed off (`git commit -s`); never squash locally; aim for < 800 changed lines.
