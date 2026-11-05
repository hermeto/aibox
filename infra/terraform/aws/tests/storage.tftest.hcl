# Weights bucket security properties (module tested in isolation, providers mocked).
mock_provider "aws" {
  mock_data "aws_caller_identity" {
    defaults = { account_id = "000000000000" }
  }
  mock_data "aws_iam_policy_document" {
    defaults = { json = "{\"Version\":\"2012-10-17\",\"Statement\":[]}" }
  }
  mock_resource "aws_kms_key" {
    defaults = { arn = "arn:aws:kms:us-east-1:000000000000:key/00000000-0000-0000-0000-000000000000", key_id = "00000000-0000-0000-0000-000000000000" }
  }
  mock_resource "aws_s3_bucket" {
    defaults = { arn = "arn:aws:s3:::aibox-weights-aibox-test-000000000000", id = "aibox-weights-aibox-test-000000000000" }
  }
}

run "weights_bucket_is_private_encrypted_and_versioned" {
  command = apply

  module {
    source = "./modules/storage"
  }

  variables {
    cluster_name = "aibox-test"
  }

  assert {
    condition     = aws_s3_bucket.weights.bucket == "aibox-weights-aibox-test-000000000000"
    error_message = "bucket name"
  }
  assert {
    condition     = aws_s3_bucket_public_access_block.weights.block_public_acls && aws_s3_bucket_public_access_block.weights.block_public_policy && aws_s3_bucket_public_access_block.weights.ignore_public_acls && aws_s3_bucket_public_access_block.weights.restrict_public_buckets
    error_message = "all public access must be blocked"
  }
  assert {
    condition     = one([for r in aws_s3_bucket_server_side_encryption_configuration.weights.rule : one(r.apply_server_side_encryption_by_default).sse_algorithm]) == "aws:kms"
    error_message = "bucket must use SSE-KMS"
  }
  assert {
    condition     = one([for r in aws_s3_bucket_server_side_encryption_configuration.weights.rule : r.bucket_key_enabled]) == true
    error_message = "S3 bucket keys must be enabled to limit KMS cost"
  }
  assert {
    condition     = one(aws_s3_bucket_versioning.weights.versioning_configuration).status == "Enabled"
    error_message = "versioning must be enabled"
  }
  assert {
    condition     = aws_kms_key.weights.enable_key_rotation
    error_message = "KMS key rotation must be enabled"
  }
  assert {
    condition     = data.aws_iam_policy_document.tls_only.statement[0].effect == "Deny" && one(data.aws_iam_policy_document.tls_only.statement[0].condition).variable == "aws:SecureTransport"
    error_message = "bucket policy must deny non-TLS access"
  }
}
