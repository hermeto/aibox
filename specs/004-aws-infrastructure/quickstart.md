# Quickstart: 004 aws-infrastructure

Offline checks (what CI runs):

```bash
cd infra/terraform/aws
tofu init -backend=false && tofu validate && tofu test
tflint --init && tflint --recursive
docker run --rm -v "$PWD:/src:ro" -w /src aquasec/trivy:0.76.1 config --severity HIGH,CRITICAL .
go test ./internal/... -run 'TestUp.*AWS|TestDown.*AWS' -race
```

Real run (maintainer only): follow [docs/runbooks/aws-first-run.md](../../docs/runbooks/aws-first-run.md).
