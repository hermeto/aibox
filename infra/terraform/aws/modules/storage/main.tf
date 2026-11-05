# Model weights bucket: SSE-KMS with a customer-managed key, versioning, TLS-only access,
# all public access blocked.
data "aws_caller_identity" "current" {}

locals {
  bucket_name = "aibox-weights-${var.cluster_name}-${data.aws_caller_identity.current.account_id}"
}

resource "aws_kms_key" "weights" {
  description             = "AIBox model weights (${var.cluster_name})"
  enable_key_rotation     = true
  deletion_window_in_days = 7
}

resource "aws_kms_alias" "weights" {
  name          = "alias/aibox-weights-${var.cluster_name}"
  target_key_id = aws_kms_key.weights.key_id
}

resource "aws_s3_bucket" "weights" {
  bucket        = local.bucket_name
  force_destroy = var.force_destroy
}

resource "aws_s3_bucket_versioning" "weights" {
  bucket = aws_s3_bucket.weights.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "weights" {
  bucket = aws_s3_bucket.weights.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = aws_kms_key.weights.arn
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_public_access_block" "weights" {
  bucket                  = aws_s3_bucket.weights.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "weights" {
  bucket = aws_s3_bucket.weights.id
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

data "aws_iam_policy_document" "tls_only" {
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    resources = [
      aws_s3_bucket.weights.arn,
      "${aws_s3_bucket.weights.arn}/*",
    ]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "weights" {
  bucket = aws_s3_bucket.weights.id
  policy = data.aws_iam_policy_document.tls_only.json

  depends_on = [aws_s3_bucket_public_access_block.weights]
}
