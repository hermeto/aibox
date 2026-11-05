# Data model: 003 local-platform

## Platform component

| Field | Source | Rule |
| --- | --- | --- |
| name | `platform/bootstrap/values-<profile>.yaml` `apps[].name` | DNS label, unique |
| path | `apps[].path` | `platform/<component>/overlays/<profile>` exists |
| namespace | `apps[].namespace` | target namespace, created by Argo CD (`CreateNamespace=true`) |
| syncWave | `apps[].syncWave` | integer; CRDs −2, operators −1, workloads 0+ |

## Root Application (created by the CLI)

`argoproj.io/v1alpha1 Application` `aibox-root` in `argocd`: source = repo URL + revision,
path `platform/bootstrap`, Helm `valueFiles: [values-<profile>.yaml]`, parameters
`repoURL`, `targetRevision`; automated sync with prune and self-heal.

## Git source

`{ repoURL, revision, mode: git|worktree, unpushedCommits: int }`.

## Endpoint

`{ name, url }` — `inference` → `http://127.0.0.1:30080/v1`, `argocd` → `http://127.0.0.1:30081`.

## Smoke report

`{ profile, endpoint, checks: [{ name, status: pass|fail, message }], ok }` — see
[contracts/smoke-report.md](contracts/smoke-report.md).
