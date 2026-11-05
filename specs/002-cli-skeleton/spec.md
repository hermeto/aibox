# Feature Specification: `aibox` CLI, profiles and preflight checks

**Feature Branch**: `002-cli-skeleton`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "The `aibox` command-line interface skeleton: command framework with consistent flags, YAML profiles validated against a JSON Schema, `aibox doctor` preflight checks for local and AWS profiles (read-only), and Makefile wrappers (playbook §6, feature 002)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discoverable, consistent commands (Priority: P1)

An operator runs `aibox --help` and sees every platform operation — `up`, `down`, `models`,
`smoke`, `bench`, `report`, `cost`, `doctor`, `version` — with the same global flags
everywhere: `--profile`, `--output json|text`, `--verbose`, `--dry-run`, `--yes`.

**Why this priority**: every later feature plugs into this command tree; the "one command"
experience starts here.

**Independent Test**: run `aibox --help` and `aibox up --profile local`; help lists all
commands and flags; `up` exits with code 2 and a "not implemented yet" message naming the
roadmap and the feature that will deliver it.

**Acceptance Scenarios**:

1. **Given** the CLI is built, **When** the operator runs `aibox --help`, **Then** the nine
   commands and five global flags are listed with one-line descriptions.
2. **Given** a command that is not implemented yet, **When** the operator runs it, **Then**
   it exits with code 2 and prints "not implemented yet" with a link to the roadmap and the
   feature number that will implement it.
3. **Given** `--output json`, **When** any command prints a result, **Then** stdout contains
   only valid JSON.
4. **Given** `aibox version`, **When** run on a release build, **Then** it prints version,
   commit and build date (injected at build time); on a development build it prints `dev`.

---

### User Story 2 - Local preflight checks (Priority: P1)

An operator runs `aibox doctor --profile local` and gets a pass/warn/fail line per check —
required tools and their minimum versions, the container runtime being reachable, and its
CPU/memory resources — each failure with a remediation hint. `--output json` emits the same
results machine-readably. The exit code is non-zero if any check fails.

**Why this priority**: the local profile is the zero-cost path for development and CI.

**Independent Test**: run `aibox doctor --profile local --output json` on the CI runner; it
exits 0 and every check is `pass` or `warn`.

**Acceptance Scenarios**:

1. **Given** all tools are installed and the container runtime is running, **When** doctor
   runs, **Then** every check passes and the exit code is 0.
2. **Given** the container runtime daemon is not running, **When** doctor runs, **Then** the
   runtime check fails with a hint to start it, and the exit code is 1.
3. **Given** a tool is older than the minimum version, **When** doctor runs, **Then** that
   check fails and names the minimum version and an install command.
4. **Given** the container runtime has less memory than recommended, **When** doctor runs,
   **Then** the resource check warns (below the hard minimum it fails).

---

### User Story 3 - Validated profiles (Priority: P1)

Profiles are YAML files in `profiles/` (`local.yaml`, `aws-dev.yaml`, `aws-bench.yaml`)
validated against a JSON Schema. Invalid profiles fail fast, naming the field path and the
reason.

**Why this priority**: every command reads a profile; a bad profile must never reach the
cloud.

**Independent Test**: load each shipped profile (valid) and a set of invalid fixtures
(unknown key, wrong type, missing required field); invalid ones fail with the JSON path.

**Acceptance Scenarios**:

1. **Given** a profile with an unknown key, **When** any command loads it, **Then** the
   command exits 1 naming the path (e.g. `/spec/clustr`) and "additional property".
2. **Given** `--profile does-not-exist`, **When** a command runs, **Then** it exits 1 listing
   the available profiles.
3. **Given** `--profile ./path/to/file.yaml`, **When** a command runs, **Then** that file is
   loaded instead of `profiles/<name>.yaml`.

---

### User Story 4 - AWS preflight checks, read-only (Priority: P2)

`aibox doctor --profile aws-dev` checks AWS prerequisites — credentials present, region set,
the profile's GPU instance types offered in the region, and the GPU vCPU service quotas for
the instance families and capacity types in the profile — using read-only calls only, and
reports "skipped: no credentials" when no credentials exist.

**Why this priority**: prevents failed or partial cloud bring-ups, but the cloud path comes
later (004+).

**Independent Test**: unit tests with a fake AWS client cover: no credentials (all AWS checks
skipped, exit 0 if local tools pass), insufficient quota (fail), instance type not offered in
region (fail), sufficient quota (pass).

**Acceptance Scenarios**:

1. **Given** no AWS credentials, **When** doctor runs for an AWS profile, **Then** AWS checks
   report `skip` with "no credentials" and no AWS API call is attempted beyond credential
   resolution.
2. **Given** credentials and a quota below what the profile needs, **When** doctor runs,
   **Then** the quota check fails with the quota name, current value, required value and a
   link to request an increase.
3. **Given** a region where a profile instance type is not offered, **When** doctor runs,
   **Then** the offering check fails before any bring-up is attempted.

---

### User Story 5 - Makefile wrappers (Priority: P2)

A contributor uses `make up|down|models-sync|smoke|bench|report|cost PROFILE=<p>` as thin
wrappers around the CLI, exactly as the product spec describes.

**Why this priority**: the product spec promises `make` entry points; they must not
duplicate CLI logic.

**Independent Test**: `make -n up PROFILE=local` shows a single `aibox up --profile local`
invocation.

**Acceptance Scenarios**:

1. **Given** `make up PROFILE=local`, **When** run, **Then** it builds the CLI and runs
   `aibox up --profile local`, propagating the exit code.

### Edge Cases

- Missing container runtime daemon → runtime check fails with a start hint.
- Profile file not found → exit 1 listing available profiles.
- Unknown profile keys → exit 1 with JSON path (schema `additionalProperties: false`).
- Linux arm64 → the CLI builds and doctor runs (no architecture-specific assumptions; tool
  version parsing is architecture-independent).
- `--output json` with an error → a JSON error object on stdout, human message on stderr.
- `--dry-run` on read-only commands (doctor, version) → no effect; on mutating commands it
  prints the planned external commands without executing them.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The CLI MUST expose commands `up`, `down`, `models`, `smoke`, `bench`, `report`,
  `cost`, `doctor`, `version` with global flags `--profile` (default `local`), `--output`
  (`text` default, `json`), `--verbose`, `--dry-run`, `--yes`.
- **FR-002**: Unimplemented commands MUST exit with code 2 and print a "not implemented yet"
  message that links the roadmap and names the delivering feature.
- **FR-003**: Profiles MUST be YAML files in `profiles/` validated against a versioned JSON
  Schema in `schemas/`; validation errors MUST name the field path and reason.
- **FR-004**: Unknown keys in profiles MUST be rejected.
- **FR-005**: `aibox doctor` MUST report each check as `pass`, `warn`, `fail` or `skip` with a
  remediation hint for `warn` and `fail`; `--output json` MUST emit a machine-readable
  report; the exit code MUST be non-zero if any check fails.
- **FR-006**: Local doctor checks MUST cover required tools with minimum versions, the
  container runtime daemon, and its CPU and memory resources against the profile's minimums.
- **FR-007**: AWS doctor checks MUST be read-only and cover credentials, region, instance
  type availability in the region, and GPU vCPU service quotas per capacity type; without
  credentials they MUST be reported as skipped.
- **FR-008**: All external commands MUST run through one command runner that supports
  dry-run and recorded fakes for tests.
- **FR-009**: Version information (version, commit, date) MUST be injectable at build time.
- **FR-010**: The Makefile MUST expose `up`, `down`, `models-sync`, `smoke`, `bench`,
  `report`, `cost` as thin wrappers around the CLI with a `PROFILE` variable.
- **FR-011**: Logs MUST go to stderr (structured), results to stdout.

### Key Entities

- **Profile**: named environment definition — provider (`kind` or `aws`), cluster settings,
  local resource minimums, AWS region and GPU classes (instance types, capacity type, node
  limit), guardrail defaults (TTL, owner).
- **Check result**: id, name, status, message, remediation, optional details.
- **Doctor report**: profile name, list of check results, summary counts, overall verdict.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Unit test coverage ≥ 80% for the profile and doctor packages.
- **SC-002**: `aibox doctor --profile local` passes on the CI runner on every PR.
- **SC-003**: `aibox up --profile local` exits with code 2 (until feature 003).
- **SC-004**: 100% of shipped profiles validate; 100% of invalid fixtures are rejected with a
  path.
- **SC-005**: Help output and doctor JSON are covered by golden files, so any change to them
  is visible in review.

## Assumptions

- Builds and tests never need cloud credentials; AWS calls are behind an interface and
  faked in tests.
- Tool minimum versions are chosen conservatively (recent stable minus one minor) and
  documented in the doctor contract.
- Budget is not part of the profile; it is required at run time (`BUDGET_USD`/`--budget`)
  by feature 005.
- The profile schema will grow with later features (models, routing, autoscaling); its
  `apiVersion` is `aibox.dev/v1alpha1`.
