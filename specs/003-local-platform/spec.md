# Feature Specification: `local` profile on kind with GitOps bootstrap

**Feature Branch**: `003-local-platform`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "One-command local platform (`aibox up --profile local`) on kind, without GPUs: kind cluster, Argo CD, app-of-apps from platform/ for the local overlay, wait until all Applications are Synced/Healthy, print endpoints; a sample model `sim-llama-8b` served by the llm-d inference simulator; CI job `e2e-local` (up → smoke → down) required by `ci-gate`; Argo CD tracks the current branch or applies from the working tree (playbook §6, feature 003)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One command brings up the local platform (Priority: P1)

A developer runs `aibox up --profile local`. A kind cluster named `aibox-local` is created,
Argo CD is installed, the root Application (app-of-apps) for the local profile is applied,
and the command waits until every Argo CD Application is Synced and Healthy, then prints the
endpoints (inference API, Argo CD UI). `aibox down --profile local` deletes the cluster.

**Why this priority**: the whole platform must be developable, demoable and testable at zero
cost; every later feature adds its local overlay on top of this.

**Independent Test**: on a 4-CPU/8-GB runner, `aibox up --profile local` completes in
≤ 10 minutes, a second `aibox up` succeeds without changes, and `aibox down` removes the
cluster.

**Acceptance Scenarios**:

1. **Given** no cluster exists, **When** the developer runs `aibox up --profile local`,
   **Then** the cluster, Argo CD and all Applications become ready and the endpoints are
   printed, within 10 minutes.
2. **Given** the platform is up, **When** `aibox up --profile local` runs again, **Then** it
   succeeds, reusing the existing cluster (idempotent).
3. **Given** the platform is up, **When** `aibox down --profile local` runs, **Then** the
   cluster is deleted; running `down` again succeeds with "nothing to delete".
4. **Given** an Application does not become Healthy before the timeout, **When** `up` waits,
   **Then** it fails naming the Application, its sync and health status and message.

---

### User Story 2 - A sample model answers chat completions (Priority: P1)

A sample model `sim-llama-8b`, served by the llm-d inference simulator (an OpenAI-compatible
fake model server that emits vLLM-style metrics), answers `POST /v1/chat/completions` on a
local endpoint. `aibox smoke --profile local` sends one request per model and checks the
response and the metrics endpoint.

**Why this priority**: proves the request path end to end without GPUs.

**Independent Test**: after `up`, `aibox smoke --profile local --output json` reports every
check as `pass`.

**Acceptance Scenarios**:

1. **Given** the platform is up, **When** a client posts a chat completion for model
   `sim-llama-8b` to the local endpoint, **Then** it receives HTTP 200 with at least one
   choice.
2. **Given** the platform is up, **When** `aibox smoke` runs, **Then** it checks
   `/v1/models` lists `sim-llama-8b`, a chat completion succeeds, and `/metrics` exposes the
   vLLM metric names AIBox depends on; any failure exits 1.

---

### User Story 3 - CI runs the local platform on every PR (Priority: P1)

The CI job `e2e-local` runs up → smoke → down on every pull request and is required by
`ci-gate`.

**Why this priority**: constitution principle III (local-first, covered by CI on every PR).

**Independent Test**: a PR shows `e2e-local` green and `ci-gate` depending on it.

**Acceptance Scenarios**:

1. **Given** a pull request, **When** CI runs, **Then** `e2e-local` creates the cluster at
   the PR's commit, runs smoke checks and deletes the cluster; `ci-gate` fails if it fails.
2. **Given** `e2e-local` fails, **When** the job ends, **Then** diagnostics (Application
   status, pod list, events) are printed before the cluster is deleted.

---

### User Story 4 - Fast iteration on local changes (Priority: P2)

By default Argo CD watches the current git branch (configurable repository URL and
revision). With `--from-worktree`, the platform manifests are applied directly from the
working tree, so uncommitted changes can be tried without pushing.

**Why this priority**: shortens the development loop but is not needed for CI.

**Independent Test**: `aibox up --profile local --from-worktree` brings the platform up from
an unpushed change; `aibox up --profile local --revision <sha>` makes Argo CD track that
revision.

**Acceptance Scenarios**:

1. **Given** uncommitted changes under `platform/`, **When** the developer runs `up
   --from-worktree`, **Then** the rendered manifests from the working tree are applied and
   the command waits for the workloads to be ready.
2. **Given** the local branch has commits not pushed to the remote, **When** `up` runs in
   git mode, **Then** it warns that Argo CD will not see unpushed commits.

### Edge Cases

- kind cluster already exists → reused (idempotent); a cluster with the same name but
  unhealthy API server → fail with a hint to run `aibox down` first.
- Docker with low memory → `up` runs the `doctor` checks first and stops on failures.
- Apple Silicon → every local component image is multi-arch (amd64 + arm64).
- Argo CD Application stuck progressing → timeout error names it, with diagnostics.
- `--dry-run` → prints the kind/helm commands and the manifests that would be applied.
- Repository URL in SSH form (`git@github.com:owner/repo.git`) → converted to HTTPS for
  Argo CD (public repositories need no credentials).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `aibox up --profile local` MUST run the doctor checks, create (or reuse) the
  kind cluster named in the profile, install Argo CD at a pinned version, apply the root
  Application for the profile, wait for every Application to be Synced and Healthy, and print
  the endpoints.
- **FR-002**: `aibox down --profile local` MUST delete the kind cluster and succeed when it
  does not exist.
- **FR-003**: Platform components MUST be declared under `platform/` as Kustomize bases with
  per-profile overlays and delivered by an Argo CD app-of-apps whose ordering uses sync waves
  (CRDs → operators → workloads).
- **FR-004**: The local profile MUST serve a sample model `sim-llama-8b` with the llm-d
  inference simulator, reachable on a local endpoint for `POST /v1/chat/completions`,
  `GET /v1/models` and `GET /metrics`.
- **FR-005**: `aibox smoke --profile local` MUST check model listing, one chat completion per
  model and the presence of required metric names, and report results in text or JSON with
  a non-zero exit code on failure.
- **FR-006**: Waiting MUST use the Kubernetes API (not the Argo CD CLI), with a configurable
  timeout, and failures MUST name the unhealthy Application with its status message.
- **FR-007**: The CI job `e2e-local` MUST run up → smoke → down on every pull request, print
  diagnostics on failure, and be required by `ci-gate`.
- **FR-008**: Argo CD MUST track a configurable repository URL and revision, defaulting to
  the current branch's remote; `--from-worktree` MUST apply manifests from the working tree.
- **FR-009**: Every local component image MUST be available for amd64 and arm64 and pinned.
- **FR-010**: `up` MUST be idempotent.

### Key Entities

- **Platform component**: a directory `platform/<component>/` with `base/` and
  `overlays/<profile>/`, delivered as one Argo CD Application with a sync wave.
- **App-of-apps**: the root Application for a profile, rendering one child Application per
  component, parameterized by repository URL and revision.
- **Endpoint**: a named URL printed after bring-up (inference API, Argo CD UI).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `e2e-local` is green in CI on every PR, completing up → smoke → down.
- **SC-002**: Bring-up completes in ≤ 10 minutes on a 4-CPU/8-GB CI runner.
- **SC-003**: A second `up` on an existing cluster succeeds (idempotency) in CI.
- **SC-004**: The README contains a "5-minute local quickstart" that a newcomer can follow.

## Assumptions

- Docker (or a compatible runtime) is available; kind, kubectl and helm are installed
  (checked by `aibox doctor`).
- The repository is public, so Argo CD can fetch it over HTTPS without credentials. Private
  forks use `--from-worktree` (documented).
- Gateway, autoscaling and observability components arrive in later features with their own
  local overlays; this feature exposes the simulator directly through a NodePort.
- Host ports 30080 (inference) and 30081 (Argo CD UI) on 127.0.0.1 are free.
