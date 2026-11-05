# Runbook: GPU nodes (Karpenter)

GPU capacity comes from Karpenter node pools generated from the profile's GPU classes
(`platform/gpu/overlays/<profile>/nodepools.yaml`, `aibox render nodepools`). Nodes run
Bottlerocket NVIDIA (`bottlerocket@v1.68.1`): the driver and the NVIDIA device plugin are part
of the AMI; the DCGM exporter runs on every GPU node.

| Pool | Instances | Capacity | Notes |
| --- | --- | --- | --- |
| `gpu-l4` | g6 | spot (aws-dev) / on-demand (aws-bench) | 1 GPU per g6.xlarge/2xlarge, 4 per g6.12xlarge |
| `gpu-a10g` | g5 | on-demand | aws-bench only |
| `gpu-h100` | p5.48xlarge | reserved (EC2 Capacity Block) | only inside the reservation window |

All pools: taint `nvidia.com/gpu=present:NoSchedule`, label `aibox.dev/gpu-class`, limits on
GPUs and vCPUs, `WhenEmpty` consolidation after 10 minutes, scale to zero, local NVMe as
ephemeral storage (`instanceStorePolicy: RAID0`) for the weight cache.

## A GPU pod stays Pending

```bash
kubectl --kubeconfig .aibox/<profile>/kubeconfig get nodeclaims
kubectl --kubeconfig .aibox/<profile>/kubeconfig describe nodeclaim <name>   # launch errors
kubectl --kubeconfig .aibox/<profile>/kubeconfig -n karpenter logs deploy/karpenter | grep -i error
```

| Message | Cause | Fix |
| --- | --- | --- |
| `InsufficientInstanceCapacity` | no capacity of that type in the AZs | add instance types to the class, or retry later (spot) |
| `VcpuLimitExceeded` / quota | EC2 vCPU quota too low | `aibox doctor` shows the required value; request an increase |
| `exceeded limits` on the NodePool | pool limit reached | raise `maxNodes` in the profile, `make generate` |
| no reserved capacity | Capacity Block not active or wrong ID | check the reservation window and the profile's `capacityReservationId` |
| pod has no toleration / wrong selector | serving manifest mismatch | `go test ./test/scheduling/...` |

## H100 via EC2 Capacity Blocks

1. Purchase a Capacity Block for `p5.48xlarge` in the profile's region (EC2 console → Capacity
   Reservations → Capacity Blocks). Note the reservation ID (`cr-…`).
2. Put it in `profiles/aws-bench.yaml` (`gpu-h100.capacityReservationId`) on the campaign
   branch and run `make generate`. The ID is account-specific (not secret); prefer a campaign
   branch that is not merged to `main`.
3. Bring the environment up shortly before the window starts; the pool can only launch inside
   the window and the instances are reclaimed at its end.

## Checking GPUs on a node

```bash
kubectl get nodes -L aibox.dev/gpu-class,karpenter.sh/capacity-type,node.kubernetes.io/instance-type
kubectl describe node <node> | grep -A2 nvidia.com/gpu
kubectl -n gpu-system port-forward ds/dcgm-exporter 9400 && curl -s localhost:9400/metrics | grep DCGM_FI_DEV_GPU_UTIL
```
