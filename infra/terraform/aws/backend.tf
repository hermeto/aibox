# Partial configuration: `aibox up` passes bucket, key and region with -backend-config.
# The state bucket is created once by ./bootstrap (see docs/runbooks/aws-first-run.md).
terraform {
  backend "s3" {
    use_lockfile = true
    encrypt      = true
  }
}
