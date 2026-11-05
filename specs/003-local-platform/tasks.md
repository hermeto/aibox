# Tasks: `local` profile on kind with GitOps bootstrap

**Input**: Design documents from `/specs/003-local-platform/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: unit tests with fakes before orchestration code; e2e in CI.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [ ] T001 Add client-go/apimachinery v0.37.2 to go.mod
- [ ] T002 [P] `infra/local/kind-config.yaml` (pinned node image, port mappings, worker label `aibox.dev/role=worker`)
- [ ] T003 [P] Local profile `kubernetesVersion: "1.37"`

## Phase 2: Foundational

- [ ] T004 [P] `platform/bootstrap/` Helm chart (Chart.yaml, values.yaml, values-local.yaml, templates/applications.yaml)
- [ ] T005 [P] `platform/bootstrap/argocd/values-local.yaml` (pinned images, insecure server NodePort 30081, Application health Lua, kustomize --enable-helm)
- [ ] T006 `internal/kube`: config from kubeconfig context, server-side apply of unstructured objects, wait for Deployments and Applications (+ tests with fake dynamic client)
- [ ] T007 [P] `internal/platform/gitsource.go`: repo URL normalization, branch, unpushed commits (+ tests)

## Phase 3: User Story 1 — up/down (P1)

- [ ] T008 [US1] Tests first: `internal/platform/local_test.go` (create vs reuse cluster, helm install args, root app object, timeout error naming the app, down when absent)
- [ ] T009 [US1] `internal/platform/local.go` orchestration (doctor → kind → Argo CD → root app → wait → endpoints)
- [ ] T010 [US1] `internal/cli/up.go`, `internal/cli/down.go` replace stubs (flags `--repo-url`, `--revision`, `--from-worktree`, `--timeout`, `--skip-doctor`)

## Phase 4: User Story 2 — sample model + smoke (P1)

- [ ] T011 [P] [US2] `platform/serving/base` (Deployment `sim-llama-8b`, Service) and `overlays/local` (NodePort 30080)
- [ ] T012 [US2] Tests first: `internal/smoke/smoke_test.go` with an httptest OpenAI-compatible server
- [ ] T013 [US2] `internal/smoke/smoke.go` and `internal/cli/smoke.go`

## Phase 5: User Story 3 — CI e2e (P1)

- [ ] T014 [US3] `test/e2e/local_test.go` (`//go:build e2e`): up → up → smoke → down, diagnostics on failure
- [ ] T015 [US3] CI job `e2e-local` (pinned kind/helm/kubectl), added to `ci-gate`; prints image digests

## Phase 6: User Story 4 — worktree mode (P2)

- [ ] T016 [US4] `--from-worktree`: render overlays with `kubectl kustomize --enable-helm`, server-side apply, wait for Deployments (+ tests)

## Phase 7: Polish

- [ ] T017 [P] ADR-0009 GitOps with Argo CD app-of-apps; ADR index
- [ ] T018 [P] README "5-minute local quickstart"; CONTRIBUTING notes
- [ ] T019 [P] `make validate-manifests` (helm template + kustomize build | kubeconform) and CI lint step
- [ ] T020 Pin the Argo CD image digest from the first e2e run
- [ ] T021 Verify lint/test/DoD; update PROGRESS.md

## Dependencies

Setup → Foundational → US1 → US2 → US3 → US4 → Polish.
