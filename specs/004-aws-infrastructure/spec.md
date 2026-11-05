# Feature Specification: AWS infrastructure as code (VPC, EKS, IAM, storage)

**Feature Branch**: `004-aws-infrastructure`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "AWS infrastructure as code for the aws-dev and aws-bench profiles: Terraform (OpenTofu-tested) for VPC (3 AZs), EKS (latest supported), a small on-demand CPU system node group, EKS Pod Identity associations, an S3 bucket for model weights, outputs consumed by the GitOps layer, tags on everything, CI validation without credentials, a remote-state bootstrap stack and a first-run runbook; `aibox up/down --profile aws-*` drive it (playbook §6, feature 004)."

## Clarifications

### Session 2026-11-05

- Q: Which Kubernetes version is "latest supported" on EKS? → A: 1.35 (newest version in eksctl v0.231.0's supported list; AWS documentation was not reachable from the build environment). The runbook tells the operator to confirm with `aws eks describe-cluster-versions` and bump the profiles if a newer version is available. (decided by agent: best verifiable source available)
- Q: Upstream modules or hand-rolled resources? → A: `terraform-aws-modules/vpc` v6.7.3, `terraform-aws-modules/eks` v21.29.0 and its `karpenter` submodule, referenced as git sources pinned to release tags; AIBox-owned modules for storage and Pod Identity roles. (decided by agent: plan input prefers the community modules; git tags work wherever GitHub is reachable and Dependabot updates them)
- Q: Which platform components get AWS identities? → A: Karpenter (controller role, node role, interruption queue via the EKS module's karpenter submodule), External Secrets (read secrets under `aibox/<cluster>/*`), the OpenTelemetry collector (read-only EC2 tags/instances for cost attribution), the model sync job (read/write the weights bucket) and model servers (read the weights bucket). KEDA gets none: it only uses the in-cluster Prometheus scaler. (decided by agent: least privilege, NFR-3; an identity with no permissions is noise)
- Q: How is the EKS public endpoint protected? → A: Public endpoint access is limited to the operator's egress IP (/32), detected by the CLI at `up` time (override with `--admin-cidr`). (decided by agent: secure by default without a VPN; no wildcard CIDR in code)
- Q: How is the weights bucket encrypted? → A: SSE-KMS with a dedicated customer-managed key (rotation on, S3 bucket key on to limit KMS request cost), versioning on, all public access blocked, TLS-only bucket policy. (decided by agent: customer-managed keys satisfy Trivy's HIGH S3 checks; bucket keys keep cost negligible)
- Q: NAT gateways? → A: One shared NAT gateway. (decided by agent: production HA is out of scope (§A.2); saves ~USD 65/month per extra NAT)
- Q: System node group size? → A: Managed node group, on-demand `m7i.xlarge`, min 2 / desired 2 / max 3. (decided by agent: room for Argo CD, Karpenter, gateway, observability and KEDA with headroom)
- Q: Where do run ID, owner and generated files live? → A: The CLI creates a run ID (`<profile>-<UTC yyyymmddhhmm>-<4 hex>`) on the first `up` and reuses it until `down`; owner is `--owner`, else the profile's `ownerEmail`, else `git config user.email`; inputs and outputs go to `.aibox/<profile>/` (git-ignored). (decided by agent: stable tags across re-runs; nothing account-specific in git)
- Q: Remote state naming and locking? → A: Bucket `aibox-tfstate-<account>-<region>` (override with `AIBOX_TF_STATE_BUCKET`), key `<profile>/<cluster>.tfstate`, S3 native locking (`use_lockfile = true`). (decided by agent: no DynamoDB table; supported by Terraform ≥ 1.10 and OpenTofu ≥ 1.10)
- Q: Which engine does the CLI run? → A: `tofu` if installed, else `terraform`; CI tests with OpenTofu. (decided by agent: the playbook targets OpenTofu while keeping Terraform compatibility)
- Q: Does `up` for AWS also bootstrap GitOps? → A: Yes: after apply it writes a kubeconfig context (`aibox-<profile>`, token via `aws eks get-token`), installs Argo CD and the root Application for `values-<profile>.yaml` (components arrive with features 006+), reusing the local bootstrap code. (decided by agent: one bring-up path for every profile)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One command creates the AWS environment (Priority: P1)

An operator runs `aibox up --profile aws-dev`. The CLI runs the preflight checks, renders the
infrastructure inputs from the profile, and applies the infrastructure: a VPC across 3
Availability Zones (private subnets for nodes, public subnets for load balancers), an EKS
cluster on the latest supported Kubernetes version, a small on-demand CPU system node group,
EKS Pod Identity associations for platform components, and an encrypted, versioned,
non-public S3 bucket for model weights. It then writes the values the GitOps layer needs to
a generated, git-ignored file and continues the bring-up. `aibox down --profile aws-dev`
destroys everything.

**Why this priority**: the cloud profiles are how the benchmark report is produced.

**Independent Test**: unit tests with a fake command runner show that `up` for an AWS profile
runs doctor, renders inputs, runs init/plan/apply in the right directory with the right
backend, reads outputs, and that `down` runs destroy. The real run is human-executed with the
runbook (bring-up to "cluster reachable" ≤ 20 minutes).

**Acceptance Scenarios**:

1. **Given** valid credentials, quotas and a state bucket, **When** the operator runs
   `aibox up --profile aws-dev --yes`, **Then** the infrastructure is created and the
   generated outputs file contains cluster name, endpoint, bucket and role ARNs.
2. **Given** `--yes` is not passed, **When** `up` runs, **Then** it shows the plan summary and
   asks for confirmation before applying; `--dry-run` only prints the commands.
3. **Given** the environment exists, **When** the operator runs `aibox down --profile
   aws-dev --yes`, **Then** the infrastructure is destroyed.
4. **Given** doctor fails (no credentials, insufficient quota, GPU types not offered in the
   region), **When** `up` runs, **Then** nothing is applied.

---

### User Story 2 - Infrastructure code is validated without cloud access (Priority: P1)

The maintainer gets CI feedback on every PR: formatting, validation, linting (AWS ruleset),
security scanning of the configuration, and tests with mocked providers that cover input
validation and key attributes (encryption, public access blocks, tags, IAM scoping).

**Why this priority**: the agent and contributors cannot run cloud applies; static and mocked
checks are the safety net.

**Independent Test**: the CI job `terraform` passes on the PR with no AWS credentials.

**Acceptance Scenarios**:

1. **Given** a PR changing infrastructure code, **When** CI runs, **Then** fmt, validate,
   tflint, trivy config and the mocked tests run and gate `ci-gate`.
2. **Given** a change that makes the weights bucket public or unencrypted, **When** tests run,
   **Then** they fail.

---

### User Story 3 - Everything is tagged (Priority: P1)

Every resource carries the tags `project=aibox`, `run-id`, `owner` and `profile`, so cost
guardrails (feature 005) can budget and detect orphans by tag.

**Why this priority**: tag-based budgets and orphan detection depend on it.

**Independent Test**: mocked tests assert the provider default tags and the tags on
representative resources; invalid tag inputs are rejected.

**Acceptance Scenarios**:

1. **Given** a run, **When** resources are planned, **Then** all four tags are present with
   the run's values.

---

### User Story 4 - Remote state bootstrap (Priority: P2)

Terraform state lives in an S3 backend with native locking, created once by a separate,
documented bootstrap stack that the operator runs by hand.

**Why this priority**: needed before the first real run, but run once.

**Independent Test**: the bootstrap stack validates and its mocked test asserts versioning,
encryption and public-access blocks on the state bucket.

**Acceptance Scenarios**:

1. **Given** a new AWS account, **When** the operator follows the runbook, **Then** the state
   bucket exists and `aibox up` uses it.

### Edge Cases

- Region without the profile's GPU families → `aibox doctor` fails before any apply.
- State bucket missing → `up` stops with the bootstrap command from the runbook.
- `down` when nothing exists → succeeds (destroy is a no-op).
- Partial apply failure → `up` exits non-zero; re-running `up` resumes (Terraform is
  idempotent); `down` destroys whatever was created.
- Account-specific values (account ID, role ARNs) never land in git: they only exist in the
  generated, git-ignored outputs file.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Infrastructure code MUST create a VPC across 3 AZs with private node subnets and
  public load-balancer subnets, an EKS cluster at the latest supported Kubernetes version, a
  small on-demand CPU system node group, Pod Identity associations (Karpenter, KEDA, OTel
  collector, External Secrets) and an S3 weights bucket (versioning, encryption, public
  access blocked).
- **FR-002**: All resources MUST carry tags `project=aibox`, `run-id`, `owner`, `profile`.
- **FR-003**: Inputs MUST be rendered from the profile (no hand-edited per-account files);
  outputs MUST be written to a generated, git-ignored file consumed by the CLI.
- **FR-004**: `aibox up --profile aws-*` MUST run doctor, init with the remote backend, plan,
  confirm (or `--yes`), apply, and read outputs; `aibox down --profile aws-*` MUST destroy.
  `--dry-run` MUST only print commands.
- **FR-005**: CI MUST validate the infrastructure code without credentials: fmt, validate,
  tflint (AWS ruleset), trivy config, and tests with mocked providers.
- **FR-006**: A separate bootstrap stack MUST create the state bucket with native locking,
  versioning, encryption and public access blocked.
- **FR-007**: A runbook MUST explain quotas, region choice, the bootstrap and the first run.
- **FR-008**: IAM permissions for platform components MUST be scoped to their needs (e.g.
  weights bucket access limited to that bucket).

### Key Entities

- **Infrastructure inputs**: cluster name, region, Kubernetes version, AZ count, system node
  group size, weights bucket name, tags (project, run-id, owner, profile).
- **Infrastructure outputs**: cluster name, endpoint, certificate authority, OIDC issuer,
  VPC/subnet IDs, weights bucket, role ARNs per component.
- **Run ID**: unique identifier per bring-up, used in tags and names.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: CI validates the infrastructure code on every PR without any AWS credential.
- **SC-002**: 100% of taggable resources carry the four tags (asserted by tests via provider
  default tags).
- **SC-003**: The runbook lets an operator reach "cluster reachable" in ≤ 20 minutes
  (human-verified).
- **SC-004**: Unit tests cover the CLI's AWS up/down command sequence (`TestUp*AWS*`).

## Assumptions

- One AWS account and one region per run; the operator has admin-equivalent credentials for
  the first run.
- OpenTofu is the tested engine; the code stays Terraform-compatible.
- GPU node pools (006), budgets/TTL (005) and multi-region are out of scope.
- The agent never runs apply/destroy; real runs are human-executed.
