# Runbook: model servers that do not become ready

Model servers are KServe `LLMInferenceService` objects in `aibox-serving`, generated from the
catalog ([ADR-0002](../adr/0002-kserve-llminferenceservice.md)). A server is ready only after its
weights are loaded and one warm-up request succeeded; until then Argo CD shows the `serving`
application as Progressing.

```bash
kubectl -n aibox-serving get llminferenceservices
```

```bash
kubectl -n aibox-serving get pods -l aibox.dev/model=<model-id>
```

```bash
kubectl -n aibox-serving logs -l aibox.dev/model=<model-id> -c main --tail=200
```

## Where it fails

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Pod `Pending`, event `didn't match Pod's node affinity/selector` or `Insufficient nvidia.com/gpu` | no GPU node yet, or the class has no instance type with enough GPUs | wait for Karpenter (`kubectl get nodeclaims`); see [gpu-nodes.md](gpu-nodes.md); rendering already rejects a tensor parallel size above the class's GPUs |
| `FailedPostStartHook`, container restarted | the warm-up request failed or loading took longer than `startupTimeoutSeconds` | read the `main` logs; raise `startupTimeoutSeconds` in the catalog for large models |
| `torch.OutOfMemoryError` / `CUDA out of memory` while loading | the model does not fit: weights + KV cache above the GPU memory | lower `gpuMemoryUtilization` or `maxModelLen`, use FP8/AWQ weights or a larger `tensorParallelSize` on a class with more GPUs |
| `NoSuchKey` / `.complete is missing` / `AccessDenied` reading `s3://…` | weights not mirrored, or the role cannot read the bucket | `aibox models sync --profile <p> --model <id>` ([model-weights.md](model-weights.md)); check the Pod Identity association of `aibox-serving/model-server` |
| `fetch-weights` init container fails (`nvme-cache`) | same as above, or the node's ephemeral storage is too small | sync the model; use a node class with instance storage |
| `401`/`403` from huggingface.co (`hf-download`) | gated model without an accepted license or token | accept the license and store the token (see [model-weights.md](model-weights.md)) |
| Pod restarts during shutdown with `SIGKILL` | in-flight requests outlived the grace period | raise `shutdownGracePeriodSeconds` |

## On AWS: weights first

`aibox up --profile aws-*` mirrors the profile's enabled models before it deploys the model
servers, so GPU nodes are not held by servers waiting for weights. If the mirror fails, `up`
stops with the failing models; fix them and run `aibox up` again (mirrored models are skipped).
`--skip-models` brings up the platform without model servers.
