# Quickstart: 001 repo-foundation

```bash
git clone https://github.com/hermeto/aibox && cd aibox
make dev-setup          # pinned linters into ./bin, pre-commit hooks
make lint test          # same checks as CI's required jobs
pre-commit run --all-files
```

Validate workflows and release config:

```bash
./bin/actionlint
go run github.com/goreleaser/goreleaser/v2@v2.20.0 check
```

Maintainer, once (admin rights): `hack/github/setup-repo.sh hermeto/aibox`.
