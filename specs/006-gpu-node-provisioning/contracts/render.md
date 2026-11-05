# Contract: `aibox render nodepools --profile <aws-*>`

Prints the Karpenter objects for the profile as YAML (deterministic order). Exit 1 for local
profiles or unknown instance types. `make generate` writes them to
`platform/gpu/overlays/<profile>/nodepools.yaml`; `go test ./internal/gpu` fails on drift.
