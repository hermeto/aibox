# EKS Pod Identity roles for AIBox platform components, each scoped to what it needs.
# KEDA has no AWS identity: it only uses the in-cluster Prometheus scaler.
data "aws_partition" "current" {}
data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  secrets_arn = "arn:${data.aws_partition.current.partition}:secretsmanager:${data.aws_region.current.region}:${data.aws_caller_identity.current.account_id}:secret:aibox/${var.cluster_name}/*"

  components = {
    external-secrets = {
      namespace       = "external-secrets"
      service_account = "external-secrets"
      statements = [{
        actions   = ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"]
        resources = [local.secrets_arn]
      }]
    }
    otel-collector = {
      namespace       = "observability"
      service_account = "otel-collector"
      statements = [{
        # Resource detection: instance type, capacity type and tags for cost attribution.
        actions   = ["ec2:DescribeInstances", "ec2:DescribeTags"]
        resources = ["*"]
      }]
    }
    model-sync = {
      namespace       = "aibox-models"
      service_account = "model-sync"
      statements = [
        {
          actions   = ["s3:ListBucket", "s3:GetBucketLocation"]
          resources = [var.weights_bucket_arn]
        },
        {
          actions   = ["s3:GetObject", "s3:PutObject", "s3:AbortMultipartUpload", "s3:ListMultipartUploadParts"]
          resources = ["${var.weights_bucket_arn}/*"]
        },
        {
          actions   = ["kms:Encrypt", "kms:Decrypt", "kms:GenerateDataKey"]
          resources = [var.weights_kms_arn]
        },
      ]
    }
    model-server = {
      namespace       = "aibox-serving"
      service_account = "model-server"
      statements = [
        {
          actions   = ["s3:ListBucket"]
          resources = [var.weights_bucket_arn]
        },
        {
          actions   = ["s3:GetObject"]
          resources = ["${var.weights_bucket_arn}/*"]
        },
        {
          actions   = ["kms:Decrypt"]
          resources = [var.weights_kms_arn]
        },
      ]
    }
  }
}

data "aws_iam_policy_document" "trust" {
  statement {
    actions = ["sts:AssumeRole", "sts:TagSession"]
    principals {
      type        = "Service"
      identifiers = ["pods.eks.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "component" {
  for_each = local.components

  dynamic "statement" {
    for_each = each.value.statements
    content {
      actions   = statement.value.actions
      resources = statement.value.resources
    }
  }
}

resource "aws_iam_role" "component" {
  for_each = local.components

  name               = "${var.cluster_name}-${each.key}"
  assume_role_policy = data.aws_iam_policy_document.trust.json
}

resource "aws_iam_role_policy" "component" {
  for_each = local.components

  name   = each.key
  role   = aws_iam_role.component[each.key].id
  policy = data.aws_iam_policy_document.component[each.key].json
}

resource "aws_eks_pod_identity_association" "component" {
  for_each = local.components

  cluster_name    = var.cluster_name
  namespace       = each.value.namespace
  service_account = each.value.service_account
  role_arn        = aws_iam_role.component[each.key].arn
}
