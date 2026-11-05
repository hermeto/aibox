# Research: 007 model-catalog

- Hugging Face Hub API: `GET https://huggingface.co/api/models/{repo}/revision/{rev}` returns
  `sha` (commit) and `cardData.license`; gated repos need a token for downloads, not for the
  revision lookup. CLI: `hf download <repo> --revision <sha> --local-dir <dir>` (huggingface_hub
  2.4.0, Python ≥ 3.10).
- s5cmd v2.3.0: `sync` uploads only missing/changed objects (resumable); `--endpoint-url` for
  S3-compatible stores; `mb` creates buckets; AWS credential chain incl. EKS Pod Identity
  (aws-sdk-go container credentials with token file).
- SeaweedFS 4.52: `weed server -dir=/data -s3 -s3.port=8333 -s3.config=<identities json>`.
  MinIO community container images are no longer published (2025), so MinIO is not used.
- External Secrets Operator v2.14.0: `ClusterSecretStore` providers `aws` (service
  `SecretsManager`, region, auth via the pod's AWS credentials) and `fake` (static data, used
  for local); `ExternalSecret` with `target.name`.
- Run:ai Model Streamer: vLLM `--load-format runai_streamer` reads `s3://` paths (008 verifies
  flags at the pinned vLLM version).
