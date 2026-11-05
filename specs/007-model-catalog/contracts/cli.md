# Contract: `aibox models`

Global flags apply (`--profile`, `--output`, `--dry-run`). `--catalog <file>` overrides
`models/catalog.yaml` (also `$AIBOX_CATALOG`).

- `aibox models list [--profile p] [--all] [-o json]` — the catalog's enabled models; filtered
  by profile only when `--profile` is given; `--all` includes disabled models. JSON:
  `{"profile","models":[<catalog entries>]}`. Exit 1 on an invalid catalog (every issue with
  its JSON path).
- `aibox models sync [--profile p] [--model id]... [--no-verify] [--timeout 30m]` — mirror
  and verify per model (every enabled model of the profile by default; explicit ids must be
  declared for the profile, else exit 64). JSON:
  `{"profile","bucket","endpoint","models":[{"id","revision","status":"synced|skipped|failed","verified","message"}]}`;
  exit 1 if any model fails. `--dry-run` lists the Jobs without touching the cluster. Cloud
  profiles need an active run (`aibox up`).
