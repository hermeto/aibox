"""AIBox model weight mirror; runs inside the Jobs rendered by internal/models/jobs.go.

mirror  Copy one Hugging Face revision to s3://$BUCKET/$PREFIX one file at a time
        (download, check the SHA-256 that Hugging Face publishes for LFS files, upload,
        delete), then write SHA256SUMS and, last, the .complete marker. A present marker
        means the mirror is complete, so the run is skipped; files already uploaded with
        a matching size and checksum are not uploaded again (resume).
verify  Stream every object listed in SHA256SUMS and compare its SHA-256 (no local disk).

The last line printed is also written to the termination log, which the CLI reports.
"""

import hashlib
import os
import shutil
import sys
import tempfile

import boto3
from botocore import UNSIGNED
from botocore.config import Config
from botocore.exceptions import ClientError

CHUNK = 8 << 20
TERMINATION_LOG = "/dev/termination-log"


def done(message, code=0):
    print(message, flush=True)
    try:
        with open(TERMINATION_LOG, "w", encoding="utf-8") as f:
            f.write(message)
    except OSError:
        pass
    sys.exit(code)


def client():
    # The local store accepts anonymous requests; on AWS, credentials come from Pod Identity.
    signature = UNSIGNED if os.environ.get("S3_ANONYMOUS") == "true" else None
    return boto3.client(
        "s3",
        endpoint_url=os.environ.get("S3_ENDPOINT") or None,
        region_name=os.environ.get("AWS_REGION") or "us-east-1",
        config=Config(
            signature_version=signature,
            retries={"max_attempts": 10, "mode": "standard"},
            s3={"addressing_style": "path"},
        ),
    )


def head(s3, bucket, key):
    try:
        return s3.head_object(Bucket=bucket, Key=key)
    except ClientError as e:
        if e.response.get("Error", {}).get("Code") in ("404", "NoSuchKey", "NotFound"):
            return None
        raise


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(CHUNK), b""):
            h.update(chunk)
    return h.hexdigest()


def ensure_bucket(s3, bucket):
    """Creates the bucket on the local store (AWS buckets come from Terraform)."""
    try:
        s3.create_bucket(Bucket=bucket)
    except ClientError as e:
        if e.response.get("Error", {}).get("Code") not in ("BucketAlreadyOwnedByYou", "BucketAlreadyExists"):
            raise


def mirror(s3, bucket, prefix):
    if os.environ.get("CREATE_BUCKET") == "true":
        ensure_bucket(s3, bucket)
    if head(s3, bucket, prefix + ".complete"):
        done("skipped: already mirrored")

    from huggingface_hub import HfApi, RepoFile, constants, hf_hub_download
    from huggingface_hub.errors import GatedRepoError, RepositoryNotFoundError, RevisionNotFoundError

    repo, revision = os.environ["HF_REPO"], os.environ["REVISION"]
    token = os.environ.get("HF_TOKEN") or None
    work = os.environ.get("WORK_DIR", "/work")
    try:
        tree = HfApi(token=token).list_repo_tree(repo, revision=revision, recursive=True)
        files = sorted((f for f in tree if isinstance(f, RepoFile)), key=lambda f: f.path)
    except GatedRepoError:
        done(f"gated: accept the license at https://huggingface.co/{repo} and store a token in the hf-token secret", 1)
    except RevisionNotFoundError:
        done(f"revision {revision} not found in {repo}", 1)
    except RepositoryNotFoundError:
        done(f"repository {repo} not found (or private: store a token in the hf-token secret)", 1)

    sums, uploaded, size = [], 0, 0
    for f in files:
        key = prefix + f.path
        expected = f.lfs.sha256 if f.lfs else None
        existing = head(s3, bucket, key)
        if existing and existing["ContentLength"] == f.size:
            stored = existing.get("Metadata", {}).get("sha256")
            if stored and (expected is None or stored == expected):
                sums.append((stored, f.path))
                continue
        with tempfile.TemporaryDirectory(dir=work) as tmp:
            local = hf_hub_download(repo, f.path, revision=revision, token=token, local_dir=tmp)
            digest = sha256_file(local)
            if expected and digest != expected:
                done(f"{f.path}: SHA-256 {digest} does not match Hugging Face ({expected})", 1)
            s3.upload_file(local, bucket, key, ExtraArgs={"Metadata": {"sha256": digest}})
        shutil.rmtree(constants.HF_XET_CACHE, ignore_errors=True)  # keep the disk at one file
        sums.append((digest, f.path))
        uploaded += 1
        size += f.size
        print(f"uploaded {f.path} ({f.size} bytes)", flush=True)

    manifest = "".join(f"{digest}  {path}\n" for digest, path in sums)
    s3.put_object(Bucket=bucket, Key=prefix + "SHA256SUMS", Body=manifest.encode())
    s3.put_object(Bucket=bucket, Key=prefix + ".complete", Body=(revision + "\n").encode())
    done(f"synced: {len(sums)} files, {uploaded} uploaded ({size} bytes)")


def verify(s3, bucket, prefix):
    if not head(s3, bucket, prefix + ".complete"):
        done(f"not mirrored: s3://{bucket}/{prefix}.complete is missing", 1)
    manifest = s3.get_object(Bucket=bucket, Key=prefix + "SHA256SUMS")["Body"].read().decode()
    lines = [line for line in manifest.splitlines() if line.strip()]
    for line in lines:
        digest, path = line.split("  ", 1)
        h = hashlib.sha256()
        for chunk in s3.get_object(Bucket=bucket, Key=prefix + path)["Body"].iter_chunks(CHUNK):
            h.update(chunk)
        if h.hexdigest() != digest:
            done(f"{path}: SHA-256 {h.hexdigest()} does not match SHA256SUMS ({digest})", 1)
    done(f"verified: {len(lines)} files")


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ("mirror", "verify"):
        done("usage: mirror.py mirror|verify", 2)
    bucket, prefix = os.environ["BUCKET"], os.environ["PREFIX"]
    s3 = client()
    try:
        (mirror if sys.argv[1] == "mirror" else verify)(s3, bucket, prefix)
    except ClientError as e:
        endpoint = os.environ.get("S3_ENDPOINT") or "AWS S3"
        done(f"object storage error ({endpoint}, bucket {bucket}): {e}", 1)


if __name__ == "__main__":
    main()
