# Runbook: model weights

How AIBox declares models, mirrors their weights into the environment's object storage and
what to do when a mirror fails. Design: [ADR-0007](../adr/0007-weight-loading-and-cold-start.md);
spec: `specs/007-model-catalog/`.

## The catalog

`models/catalog.yaml` declares every model: source repository and **pinned commit**, license,
quantization, tensor parallel size, context length, minimum GPU class, runtime and flags,
loading method and SLO targets. It is validated against `schemas/catalog.schema.json`.

```bash
aibox models list
```

```bash
aibox models list --profile aws-bench --output json
```

`--all` includes disabled models (for example gated ones).

### Pin or bump a revision

The build environment of the AIBox agent cannot reach Hugging Face, so new entries use the
placeholder revision `0000000000000000000000000000000000000000`. CI (`make
check-model-revisions`, `hack/hfcheck`) fails on placeholders and prints the commit to pin
(the current head of `main`); it also fails when a pinned revision no longer exists. With
network access you can run it locally:

```bash
make check-model-revisions
```

## Mirror the weights

```bash
aibox models sync --profile local --model tiny-random-llama
```

```bash
aibox models sync --profile aws-bench
```

Without `--model`, every enabled model of the profile is synced. For each model the CLI
applies two Jobs in the `aibox-models` namespace:

1. `mirror-<id>` downloads the pinned revision file by file, checks each file against the
   SHA-256 that Hugging Face publishes for large (LFS) files, uploads it to
   `s3://<bucket>/models/<id>/<revision>/`, then writes `SHA256SUMS` and, last, `.complete`.
   A model whose `.complete` marker exists is **skipped**; a mirror that was interrupted
   **resumes** (files already uploaded with the same size and checksum are not uploaded
   again).
2. `verify-<id>` (skip with `--no-verify`) streams every object back from the bucket and checks
   it against `SHA256SUMS`: proof that a pod can load the model.

| Profile | Bucket | Credentials |
| --- | --- | --- |
| local | `aibox-weights` on the in-cluster SeaweedFS store (`seaweedfs.aibox-models:8333`) | none (anonymous; reachable only inside kind) |
| aws-* | `weights_bucket` Terraform output (`aibox-weights-<cluster>-<account>`) | EKS Pod Identity role `<cluster>-model-sync` |

The local store keeps data in an `emptyDir`: after `aibox down` / `up`, sync again.

## Gated models and the Hugging Face token

Gated models (for example `llama-3.1-8b-instruct`) ship disabled. To use one:

1. Accept the license on the model page on huggingface.co with your account.
2. Create a read token on huggingface.co (Settings → Access Tokens).
3. Store it in AWS Secrets Manager under `aibox/<cluster>/hf-token` (plain text), in the
   profile's region:

   ```bash
   aws secretsmanager create-secret --region us-east-1 --name aibox/aibox-bench/hf-token --secret-string "$HF_TOKEN"
   ```

4. Set `enabled: true` for the model in your copy of the catalog and run `aibox models sync`.

External Secrets copies the token into the `hf-token` Secret of `aibox-models` within an hour
(`refreshInterval`). While the key does not exist the Secret is absent and the Jobs download
anonymously. Never put a token in git or in a profile.

## When a sync fails

The CLI prints the Job's message and the command to read its logs:

```bash
kubectl -n aibox-models logs job/mirror-<id>
```

| Message | Cause | Fix |
| --- | --- | --- |
| `revision not pinned` | placeholder revision in the catalog | pin the commit printed by `make check-model-revisions` |
| `gated: accept the license …` | license not accepted or no token | follow "Gated models" above |
| `revision … not found` | the commit was removed upstream | pin a new revision; CI flags this too |
| `SHA-256 … does not match` | corrupted download or object | run the sync again (only the bad file is re-uploaded) |
| `object storage error (…)` | store unreachable, bucket missing or access denied | check `aibox up` finished; on AWS check the Pod Identity association of `model-sync` |
| `timed out` | very large model or slow network | `--timeout 2h`; a rerun resumes |

To force a full re-mirror of one revision, delete its prefix (AWS:
`aws s3 rm --recursive s3://<bucket>/models/<id>/<revision>/`) and sync again.
