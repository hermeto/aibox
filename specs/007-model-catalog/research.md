# Research: 007 model-catalog

- Hugging Face Hub API: `GET https://huggingface.co/api/models/{repo}/revision/{rev}` returns
  `sha` (commit) and `cardData.license`; gated repos need a token for downloads, not for the
  revision lookup. CLI: `hf download <repo> --revision <sha> --local-dir <dir>` (huggingface_hub
  2.4.0, Python ≥ 3.10).
- huggingface_hub 2.4.0 (source read from the wheel): `HfApi.list_repo_tree(repo, revision=…,
  recursive=True)` yields `RepoFile` (`path`, `size`, `lfs.sha256` for LFS files) and
  `RepoFolder`; `hf_hub_download(repo, filename, revision=…, local_dir=…)`; errors in
  `huggingface_hub.errors` (`GatedRepoError`, `RepositoryNotFoundError`,
  `RevisionNotFoundError`); the Xet chunk cache lives in `constants.HF_XET_CACHE`.
- boto3 1.43: S3 client with `endpoint_url`, path-style addressing and `UNSIGNED` signatures
  for the anonymous local store; the default credential chain covers EKS Pod Identity
  (container credentials with a token file). `upload_file` does managed multipart uploads.
- s5cmd v2.3.0 was evaluated first (`sync` is resumable) and dropped: its image is Alpine
  with a static binary, so using it next to Python meant copying a binary between images.
- SeaweedFS 4.52 (`docker/Dockerfile.go_build`, `docker/entrypoint.sh` at tag 4.52): binary
  `/usr/bin/weed`, user `seaweed` (uid 1000); `weed server -dir=/data -s3 -s3.port=8333`
  without `-s3.config` serves S3 without identities (anonymous). MinIO community container
  images are no longer published (2025), so MinIO is not used.
- External Secrets Operator v2.14.0 (chart tag `helm-chart-2.14.0`, CRDs in
  `config/crds/bases`): API `external-secrets.io/v1`; `ClusterSecretStore` providers `aws`
  (`service: SecretsManager`, `region`; no `auth` means the controller's own credentials, here
  Pod Identity) and `fake` (`data[].key/value`). In
  `pkg/controllers/externalsecret/externalsecret_controller{,_secret}.go`, a missing provider
  secret is skipped unless `target.deletionPolicy: Retain`, and with `Delete` an empty result
  deletes the target Secret and marks the ExternalSecret ready (`SecretDeleted`).
- Run:ai Model Streamer: vLLM `--load-format runai_streamer` reads `s3://` paths (008 verifies
  flags at the pinned vLLM version).
