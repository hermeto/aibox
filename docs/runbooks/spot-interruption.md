# Runbook: spot interruptions

Spot GPU nodes (e.g. `gpu-l4` on aws-dev) can be reclaimed with a two-minute warning.
Karpenter watches the interruption queue created by Terraform (`Karpenter-<cluster>`,
EventBridge rules for spot interruption, rebalance, state change and scheduled events) and
cordons and drains the node before reclamation, then provisions a replacement.

## What happens to inference

- In-flight requests on the drained pod are lost when the pod stops; graceful shutdown
  (feature 008) drains requests up to the termination grace period, which is shorter than
  the two-minute warning.
- The replacement node needs image pull + weight load (cold start, see E7) — p99 latency
  rises during the gap. Experiment E6 measures this.

## Investigate

```bash
kubectl get events -A --field-selector reason=SpotInterrupted
kubectl -n karpenter logs deploy/karpenter | grep -i interruption
```

## Reduce impact

- Run at least two replicas per model on spot.
- Mix instance types in the class (more spot pools → fewer interruptions).
- Use on-demand (`capacityType: on-demand`) for latency-critical benchmarks.
