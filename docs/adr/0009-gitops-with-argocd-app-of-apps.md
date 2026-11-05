# ADR-0009: GitOps with Argo CD app-of-apps

- **Status**: Accepted
- **Date**: 2026-11-05
- **Deciders**: maintainer (decision taken by the agent on the maintainer's behalf)
- **Decided by**: feature 003

## Context and problem statement

Constitution principle V requires platform state to live in git under `platform/` and be
delivered declaratively, with imperative steps limited to an idempotent bootstrap. The same
mechanism must work on a laptop/CI (`local`, kind) and on EKS (`aws-*`), and every component
needs an ordering (CRDs → operators → workloads).

## Decision drivers

- Works identically for `local` and AWS profiles; zero cost locally.
- Ordering between components; health-gated rollout.
- Reviewable diffs: what runs is what is in git at a known revision (benchmark metadata).
- Low operational overhead for a single maintainer; widely known tooling.

## Considered options

1. Argo CD app-of-apps (root Application rendering one child Application per component).
2. Argo CD ApplicationSets (git directory generator).
3. Flux (Kustomization + HelmRelease).
4. CLI-driven `kubectl apply`/`helm install` only (no GitOps controller).

## Decision outcome

Chosen option: "Argo CD app-of-apps", because it gives explicit, per-component sync waves
with health gating (with the documented Application health customization), a UI that makes
the platform's state visible in demos, and a root Application that the CLI can parameterize
per run (repository URL and exact revision) — the revision is recorded in benchmark
metadata. The root renders a tiny Helm chart (`platform/bootstrap`, `values-<profile>.yaml`);
each child points to `platform/<component>/overlays/<profile>`, plain Kustomize with
`--enable-helm` for pinned upstream charts.

### Consequences

- Good: one mechanism for every profile; bootstrap is three idempotent steps (cluster,
  Argo CD Helm release, root Application).
- Good: drift is visible and self-healed.
- Bad: Argo CD must reach the git repository; unpushed changes need `--from-worktree`
  (direct apply), and private forks need repository credentials or worktree mode.
- Bad: Argo CD itself (~5 pods) consumes part of the local runner's 4 CPUs / 8 GB.

### Confirmation

CI job `e2e-local` brings the platform up through Argo CD at the PR's exact commit, waits for
every Application to be Synced/Healthy, and runs smoke checks.

## Pros and cons of the options

### ApplicationSets

- Good: no chart for the root. Bad: per-run parameterization (revision) and per-component
  waves are less direct.

### Flux

- Good: lighter footprint. Bad: no built-in UI for demos; the team-facing ecosystem
  (llm-d/KServe guides) documents Argo CD more often.

### CLI-only apply

- Good: simplest. Bad: violates principle V (no reconciliation, no drift detection).

## Links

- [specs/003-local-platform/research.md](../../specs/003-local-platform/research.md)
- Argo CD cluster bootstrapping: <https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/>
