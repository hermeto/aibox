# Contract: `aibox up|down --profile local`

## `aibox up`

| Flag | Default | Meaning |
| --- | --- | --- |
| `--repo-url` | `origin` remote (HTTPS) | Git repository Argo CD tracks |
| `--revision` | current branch | branch, tag or commit SHA Argo CD tracks |
| `--from-worktree` | false | apply rendered overlays from the working tree instead of the root Application |
| `--timeout` | `10m` | maximum wait for Argo CD and Applications |
| `--skip-doctor` | false | skip the preflight checks |

Steps (each idempotent): doctor → kind cluster (create if absent) → Argo CD (`helm upgrade
--install`) → wait for Argo CD Deployments → root Application (server-side apply) → wait for
all Applications Synced/Healthy → print endpoints.

Output (text): one line per step, then endpoints. JSON:
`{"profile":"local","cluster":"aibox-local","source":{…},"endpoints":[{"name":"inference","url":"http://127.0.0.1:30080/v1"},{"name":"argocd","url":"http://127.0.0.1:30081"}]}`.

Exit: 0 ready; 1 failure (doctor fail, timeout, apply error) with the failing Application's
status in the message.

## `aibox down`

Deletes the kind cluster named in the profile. Exit 0 if it did not exist ("nothing to
delete"). For AWS profiles: not implemented (exit 2) until feature 004.
