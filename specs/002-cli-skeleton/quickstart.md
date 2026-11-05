# Quickstart: 002 cli-skeleton

```bash
go run ./cmd/aibox --help
go run ./cmd/aibox doctor --profile local
go run ./cmd/aibox doctor --profile local --output json | jq .
go run ./cmd/aibox up --profile local; echo "exit=$?"   # exit=2 until feature 003
go run ./cmd/aibox doctor --profile aws-dev              # AWS checks "skip" without credentials
make build && ./bin/aibox version
make -n up PROFILE=local                                 # thin wrapper around the CLI
go test ./... -race -cover
```
