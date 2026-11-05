# Research: 001 repo-foundation

All versions resolved on 2026-11-05 from upstream git tags (`git ls-remote --tags`), the Go
module proxy, PyPI and npm. Actions are pinned by the commit SHA the release tag points to.

## Pinned GitHub Actions

| Action | Version | Commit SHA |
| --- | --- | --- |
| actions/checkout | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| actions/setup-go | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| astral-sh/setup-uv | v10.3.3 | `68559e99cc807c1eb7e9238fd43a15bba460eec2` |
| actions/setup-node | v7.1.0 | `949feb2413d6458794dcd2491c4babbbce0c15c1` |
| actions/cache | v6.1.0 | `55cc8345863c7cc4c66a329aec7e433d2d1c52a9` |
| actions/dependency-review-action | v5.0.0 | `a1d282b36b6f3519aa1f3fc636f609c47dddb294` |
| amannn/action-semantic-pull-request | v6.1.1 | `48f256284bd46cdaab1048c3721360e808335d50` |
| github/codeql-action | v4.39.3 | `33d7b85da1a04171e211befe2c739303b9a4c066` |
| ossf/scorecard-action | v2.5.0 | `fa7117db83c52d787ae5a89f350999701505d78e` |
| actions/upload-artifact | v7.0.2 | `cf430e030ddbb5b0abf93d22962f4752f3646cd9` |
| googleapis/release-please-action | v5.0.0 | `45996ed1f6d02564a971a2fa1b5860e934307cf7` |
| goreleaser/goreleaser-action | v7.3.0 | `a4f02ab5355a589de60d917a659ce617ed19b3de` |
| sigstore/cosign-installer | v4.1.3 | `756fe2d284c7292c4e9b62c8a28f1e5629c59532` |
| anchore/sbom-action (download-syft) | v0.24.3 | `66cbf4bc1f1c0d2edc94016e65bc221b6bb0ad6c` |
| actions/attest-build-provenance | v4.2.2 | `4d101475d8b20a2381f78447822ac1eab6504dd8` |
| crazy-max/ghaction-github-labeler | v6.0.0 | `548a7c3603594ec17c819e1239f281a3b801ab4d` |

Inputs were checked against each pinned `action.yml` (e.g. setup-go `go-version-file`,
dependency-review `fail-on-severity`, labeler `yaml-file`/`skip-delete`).

## Pinned tools

| Tool | Version | Install path |
| --- | --- | --- |
| Go | 1.27.3 | `go.mod` (`go 1.27.3`), `setup-go` reads `go-version-file` |
| golangci-lint | v2.15.0 | `go install` into `./bin` |
| actionlint | v1.7.12 | `go install` into `./bin` |
| editorconfig-checker | v3.11.3 | `go install` into `./bin` |
| shellcheck | 0.11.0 (shellcheck-py 0.11.0.1) | `uvx --from shellcheck-py==0.11.0.1 shellcheck` |
| yamllint | 1.38.0 | `uvx yamllint==1.38.0` |
| markdownlint-cli2 | 0.24.0 | `npx --yes markdownlint-cli2@0.24.0` |
| pre-commit | 4.7.0 | `uv tool install pre-commit==4.7.0` (dev-setup) |
| Trivy | 0.76.1 | `aquasec/trivy:0.76.1@sha256:790fd66d1697395d8693d1acaa1ea702406a20f109b6fc7bac8db623eca6c06c` |
| GoReleaser | v2.20.0 | `goreleaser-action` `version: v2.20.0` |
| syft | v1.56.0 | `anchore/sbom-action/download-syft` `syft-version: v1.56.0` |
| cosign | v3.1.4 | `cosign-installer` `cosign-release: v3.1.4` |

## Decisions

### D1 — One Makefile as the single entry point

- **Decision**: All linters run through `make lint-*` targets; CI and pre-commit call them.
- **Rationale**: guarantees "pre-commit mirrors CI" and works without Homebrew.
- **Alternatives**: separate actions per linter (drift between local and CI); pre-commit as
  the only runner (slower in CI, less transparent for newcomers).

### D2 — `ci-gate` aggregation

- **Decision**: `ci-gate` uses `needs:` on all required jobs, `if: always()`, and a script
  (`hack/ci/ci-gate.sh`) that fails unless every result in `toJSON(needs)` is `success`.
- **Rationale**: one stable required check while later features add jobs (they only extend
  `needs:`).
- **Alternatives**: listing each job as required in the ruleset (brittle as jobs change).

### D3 — DCO without an app

- **Decision**: `hack/ci/dco-check.sh <base> <head>` walks non-merge commits and requires a
  `Signed-off-by:` trailer whose email matches the author email.
- **Alternatives**: DCO GitHub App (third-party install, excluded by plan input).

### D4 — Trivy via pinned container image

- **Decision**: `docker run aquasec/trivy@sha256:…` with `fs` and `config` scans, failing on
  HIGH/CRITICAL fixable findings.
- **Rationale**: avoids the action wrapper whose tags were tampered with in the 2026
  incident; digest pinning is stronger than a tag.

### D5 — Release automation

- **Decision**: release-please (`release-type: go`, manifest mode) opens release PRs from
  Conventional Commits; `release.yml` runs only on `v*` tags and uses GoReleaser to build
  darwin/linux × amd64/arm64 archives, syft SBOMs, keyless cosign signatures of the checksum
  file (OIDC), and `actions/attest-build-provenance` for SLSA provenance.
- **Alternatives**: semantic-release (Node-centric), manual changelog (violates FR-009).

### D6 — Labels and repository settings

- **Decision**: `.github/labels.yml` + `crazy-max/ghaction-github-labeler` on push to the
  default branch (`skip-delete: true`, so GitHub defaults that are not in the catalog stay
  until the maintainer removes them); `hack/github/setup-repo.sh` applies everything that
  needs admin rights once.
- **Rationale**: the agent cannot change admin settings in this run.

### D7 — CodeQL languages

- **Decision**: `go` and `actions` now; `python` joins in feature 014.
- **Rationale**: CodeQL errors on a language without source files.

## References

- GitHub rulesets REST API: <https://docs.github.com/en/rest/repos/rules>
- Contributor Covenant 2.1: <https://www.contributor-covenant.org/version/2/1/code_of_conduct/>
- OpenSSF Scorecard action: <https://github.com/ossf/scorecard-action>
- GoReleaser signing and SBOMs: <https://goreleaser.com/customization/sign/>, <https://goreleaser.com/customization/sbom/>
- Artifact attestations: <https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations>
- MADR: <https://adr.github.io/madr/>
