# Quickstart: 003 local-platform

```bash
make build
./bin/aibox up --profile local          # kind + Argo CD + app-of-apps, ~5 minutes
curl -s http://127.0.0.1:30080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"sim-llama-8b","messages":[{"role":"user","content":"Hello"}]}' | jq .
./bin/aibox smoke --profile local
./bin/aibox down --profile local
```

Iterate on unpushed changes: `./bin/aibox up --profile local --from-worktree`.
Run the e2e test like CI: `go test -tags e2e ./test/e2e/... -run TestLocalPlatform -timeout 30m -v`.
