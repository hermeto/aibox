# Contract: `ci-gate` status check

- **Name**: `ci-gate` (job id and display name), reported on every pull request (any base
  branch) and on pushes to the default branch.
- **Inputs**: the results of all required jobs via `needs`.
- **Behavior**: runs with `if: always()`; exits 0 only if every needed job result is
  `success`; otherwise prints a table of `job → result` and exits 1.
- **Required jobs (feature 001)**: `lint`, `test`, `pr-title` (pull requests only),
  `dco` (pull requests only), `dependency-review` (pull requests only), `trivy`.
  On `push` events the pull-request-only jobs are skipped by design, so `ci-gate` evaluates
  only the jobs that apply to the event (`hack/ci/ci-gate.sh` receives the allowed-skip
  list `pr-title,dco,dependency-review` on push events).
- **Extension rule**: later features add their required jobs to `needs:`; the ruleset never
  changes.
