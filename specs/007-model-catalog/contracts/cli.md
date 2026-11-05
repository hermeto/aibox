# Contract: `aibox models`

- `aibox models list [--profile p] [--all] [-o json]` — enabled models for the profile.
- `aibox models sync [--profile p] [--model id]... [--no-verify] [--timeout 30m]` — mirror +
  verify per model; JSON: `{"profile","bucket","models":[{"id","revision","status":"synced|skipped|failed","message"}]}`; exit 1 if any fails.
