# Contract: Makefile targets

| Target | Behavior | Exit code |
| --- | --- | --- |
| `make help` | lists every documented target (`## comment` convention) | 0 |
| `make dev-setup` | installs pinned tools (`make tools`), checks `go`, `uv`, `node`, `docker`, installs pre-commit hooks | non-zero if a mandatory prerequisite is missing, with the install command |
| `make tools` | installs Go-based linters into `./bin` at pinned versions | 0 |
| `make lint` | runs `lint-go`, `lint-actions`, `lint-shell`, `lint-yaml`, `lint-markdown`, `lint-editorconfig` | non-zero on any finding |
| `make fmt` | `gofmt`/`goimports` via golangci-lint `fmt` | 0 |
| `make test` | `go test -race -cover ./...` + shell tests under `hack/` | non-zero on failure |

No target requires cloud credentials.
