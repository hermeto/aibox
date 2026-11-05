# Research: 006 gpu-node-provisioning

- Karpenter v1.15.1 (`aws/karpenter-provider-aws`), CRDs read from `pkg/apis/crds/` at the
  tag: `EC2NodeClass.spec.capacityReservationSelectorTerms[].id`, `instanceStorePolicy: RAID0`,
  `amiSelectorTerms[].alias` (`bottlerocket@<version>`), `metadataOptions`;
  `NodePool.spec.disruption.consolidationPolicy` ∈ {WhenEmpty, WhenEmptyOrUnderutilized,
  Balanced}, `consolidateAfter`, `limits`, `template.spec.taints`, `requirements` with
  `karpenter.sh/capacity-type` ∈ {spot, on-demand, reserved}. Chart values:
  `settings.clusterName`, `settings.interruptionQueue`, `settings.featureGates.reservedCapacity`
  (beta, default on). Source: <https://github.com/aws/karpenter-provider-aws/tree/v1.15.1/pkg/apis/crds>.
- Controller image index digest `sha256:8314ff9c6bca66372031e80532c18025bbe6d34eae2443849d9ff02796fb09c8`
  (`docker buildx imagetools inspect public.ecr.aws/karpenter/controller:1.15.1`).
- Bottlerocket v1.68.1 (latest tag); NVIDIA variants include the driver and the Kubernetes
  device plugin: <https://bottlerocket.dev/en/os/latest/#/concepts/variants/>.
- DCGM exporter chart 4.8.3 / image `4.6.0-4.8.3-distroless` (repo `NVIDIA/dcgm-exporter`,
  `deployment/`); digest pinned from the CI digest check (nvcr.io unreachable from the
  build environment).
- Instance table (GPUs, vCPUs) used for limits: g6.xlarge 1/4, g6.2xlarge 1/8, g6.12xlarge
  4/48, g5.xlarge 1/4, g5.12xlarge 4/48, p5.48xlarge 8/192 (AWS instance type specs).
