# Feature Specification: Just-in-time GPU nodes with Karpenter and GPU telemetry

**Feature Branch**: `006-gpu-node-provisioning`

**Created**: 2026-11-05

**Status**: Draft

**Input**: User description: "Karpenter GPU NodePools per GPU class (gpu-l4/g6, gpu-a10g/g5, gpu-h100/p5 with an EC2 Capacity Block), tainted and labeled, with per-pool limits and capacity type from the profile; WhenEmpty consolidation; NVIDIA-enabled AMI with local NVMe for the weight cache; GPU resources and DCGM metrics; scale to zero; a local overlay with a fake GPU node on kind (playbook §6, feature 006)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One node pool per GPU class (Priority: P1)

The operator gets one Karpenter NodePool per GPU class declared in the profile — `gpu-l4`
(g6), `gpu-a10g` (g5), `gpu-h100` (p5, only through an EC2 Capacity Block reservation) — each
tainted `nvidia.com/gpu:NoSchedule`, labeled `aibox.dev/gpu-class=<class>`, limited in GPUs
and vCPUs, with the capacity type (spot or on-demand) taken from the profile.

**Why this priority**: GPU capacity is the dominant cost; nodes must exist only for GPU pods.

**Independent Test**: the node pools rendered for `aws-dev` and `aws-bench` validate against
the pinned Karpenter CRD schemas and match the profiles' GPU classes (drift test).

**Acceptance Scenarios**:

1. **Given** the aws-bench profile, **When** node pools are rendered, **Then** there are three
   pools with the class label, the GPU taint, the instance families of the profile and limits
   = maxNodes × GPUs/vCPUs of the largest instance type.
2. **Given** a profile class with `capacityType: spot`, **When** rendered, **Then** the pool
   requires `karpenter.sh/capacity-type=spot`.
3. **Given** the H100 class, **When** rendered, **Then** the pool only allows reserved
   capacity and its node class selects the reservation ID from the profile.

---

### User Story 2 - Warm nodes are not consolidated away (Priority: P1)

GPU pools consolidate only when empty (`WhenEmpty`) after a configurable `consolidateAfter`,
so pods holding a warm KV-cache are never evicted for bin-packing; pools scale to zero when no
GPU pod exists.

**Why this priority**: evicting a warm model costs minutes of cold start (experiment E7).

**Independent Test**: rendered pools have `consolidationPolicy: WhenEmpty` and the configured
`consolidateAfter`; there is no static GPU capacity (min 0).

**Acceptance Scenarios**:

1. **Given** any GPU pool, **When** rendered, **Then** disruption is `WhenEmpty` with
   `consolidateAfter` from the pool settings (default 10m).

---

### User Story 3 - NVIDIA-ready nodes with a local weight cache (Priority: P1)

GPU nodes boot an NVIDIA-enabled AMI with the driver and device plugin preinstalled and use
the instance's local NVMe for ephemeral storage (model weight cache); GPU resources
(`nvidia.com/gpu`) and DCGM metrics are available.

**Why this priority**: runtimes need GPUs, and E7 compares loading from the NVMe cache.

**Independent Test**: the node class pins an NVIDIA-capable AMI alias with a version and sets
the instance-store policy; the DCGM exporter is part of the aws overlays with GPU tolerations.

**Acceptance Scenarios**:

1. **Given** the aws overlays, **When** rendered, **Then** the EC2NodeClass uses a pinned AMI
   alias, RAID0 instance store, IMDSv2, the Karpenter node role and subnet/security-group
   discovery by cluster tag; a DCGM exporter DaemonSet targets GPU nodes.

---

### User Story 4 - GPU scheduling is testable locally (Priority: P1)

On kind, one worker carries the fake GPU label and taint so that serving pods' tolerations
and node selectors are exercised without GPUs.

**Why this priority**: constitution principle III.

**Independent Test**: `aibox up --profile local` shows a node with
`aibox.dev/gpu-class=gpu-sim`, and the sample model runs on it.

**Acceptance Scenarios**:

1. **Given** the local platform, **When** listing nodes with the label column, **Then** one
   worker shows `gpu-sim`, and the simulator pod is scheduled there.

### Edge Cases

- Spot interruption → Karpenter's interruption queue (from 004) drains the node before
  reclamation; documented in `docs/runbooks/spot-interruption.md`.
- Quota exhausted → NodeClaims stay pending with an `InsufficientCapacity`/quota event;
  `aibox doctor` warns before the run; runbook `docs/runbooks/gpu-nodes.md`.
- Capacity Block not active yet → the H100 pool cannot launch; pods stay pending (no
  on-demand fallback by design).
- A profile class without a matching pool (drift) → CI test fails.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Each GPU class in a cloud profile MUST have one Karpenter NodePool with taint
  `nvidia.com/gpu:NoSchedule`, label `aibox.dev/gpu-class`, instance-family requirements,
  capacity-type requirement from the profile, and limits for `nvidia.com/gpu` and `cpu`.
- **FR-002**: GPU pools MUST use `WhenEmpty` consolidation with a configurable
  `consolidateAfter` and no minimum capacity (scale to zero).
- **FR-003**: Node classes MUST use a pinned NVIDIA-capable AMI, the instance store for
  ephemeral storage (RAID0), IMDSv2 and tag-based subnet/security-group discovery.
- **FR-004**: The H100 class MUST only use reserved capacity from the profile's Capacity
  Block reservation.
- **FR-005**: Karpenter MUST be installed from its pinned chart with the cluster name and the
  interruption queue from feature 004.
- **FR-006**: A DCGM exporter MUST run on GPU nodes and expose GPU metrics for the
  observability stack.
- **FR-007**: The local profile MUST provide a worker with the fake GPU label and taint, and
  the sample model MUST tolerate and select it.
- **FR-008**: Manifests MUST validate with kubeconform against CRD schemas extracted from the
  pinned charts, and the rendered pools MUST match the profiles (drift check in CI).

### Key Entities

- **GPU class**: name, instance types, capacity type, max nodes, optional reservation ID
  (from the profile).
- **Node pool / node class**: Karpenter objects rendered from GPU classes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `make validate-manifests` validates local and aws overlays, including Karpenter
  objects, in CI.
- **SC-002**: A unit test asserts that serving pods' tolerations/selectors match the pools'
  taints/labels for every profile.
- **SC-003**: The rendered pools are byte-identical to the committed ones (no drift).
- **SC-004**: Runbook `docs/runbooks/gpu-nodes.md` covers quotas, launch failures and
  Capacity Blocks.

## Assumptions

- Launching real GPU nodes is human-run (runbook); CI validates manifests only.
- Pending-pod-to-node-ready timings (FR-AS-6) are measured by feature 012's scale timer.
