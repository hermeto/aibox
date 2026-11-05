mock_provider "aws" {
  mock_data "aws_caller_identity" {
    defaults = { account_id = "000000000000" }
  }
  mock_data "aws_iam_policy_document" {
    defaults = { json = "{\"Version\":\"2012-10-17\",\"Statement\":[]}" }
  }
  mock_resource "aws_kms_key" {
    defaults = { arn = "arn:aws:kms:us-east-1:000000000000:key/00000000-0000-0000-0000-000000000000" }
  }
  mock_resource "aws_s3_bucket" {
    defaults = { arn = "arn:aws:s3:::aibox-tfstate-000000000000-us-east-1" }
  }
}

variables {
  region = "us-east-1"
  owner  = "maintainer@example.com"
}

run "state_bucket_is_protected" {
  command = plan

  assert {
    condition     = output.state_bucket == "aibox-tfstate-000000000000-us-east-1"
    error_message = "state bucket name must match what aibox up derives"
  }
  assert {
    condition     = one(aws_s3_bucket_versioning.state.versioning_configuration).status == "Enabled"
    error_message = "state bucket must be versioned"
  }
  assert {
    condition     = aws_s3_bucket_public_access_block.state.restrict_public_buckets && aws_s3_bucket_public_access_block.state.block_public_policy
    error_message = "state bucket must block public access"
  }
  assert {
    condition     = one([for r in aws_s3_bucket_server_side_encryption_configuration.state.rule : one(r.apply_server_side_encryption_by_default).sse_algorithm]) == "aws:kms"
    error_message = "state bucket must use SSE-KMS"
  }
}
