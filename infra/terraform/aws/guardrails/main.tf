# Cost guardrails for one run: a tag-scoped budget with e-mail alerts and a one-time TTL
# teardown (EventBridge Scheduler → CodeBuild running `tofu destroy`). See ADR-0010.
data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

locals {
  name       = "aibox-${var.run_id}"
  account_id = data.aws_caller_identity.current.account_id
  partition  = data.aws_partition.current.partition
}

# ---- Alerts -------------------------------------------------------------------------------

resource "aws_kms_key" "alerts" {
  description             = "AIBox alerts topic (${var.run_id})"
  enable_key_rotation     = true
  deletion_window_in_days = 7
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AccountAdmin"
        Effect    = "Allow"
        Principal = { AWS = "arn:${local.partition}:iam::${local.account_id}:root" }
        Action    = "kms:*"
        Resource  = "*"
      },
      {
        Sid       = "AllowBudgetsToPublish"
        Effect    = "Allow"
        Principal = { Service = "budgets.amazonaws.com" }
        Action    = ["kms:GenerateDataKey*", "kms:Decrypt"]
        Resource  = "*"
      },
    ]
  })
}

resource "aws_sns_topic" "alerts" {
  name              = "${local.name}-alerts"
  kms_master_key_id = aws_kms_key.alerts.arn
}

resource "aws_sns_topic_policy" "alerts" {
  arn = aws_sns_topic.alerts.arn
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "AllowBudgets"
      Effect    = "Allow"
      Principal = { Service = "budgets.amazonaws.com" }
      Action    = "SNS:Publish"
      Resource  = aws_sns_topic.alerts.arn
      Condition = { StringEquals = { "aws:SourceAccount" = local.account_id } }
    }]
  })
}

resource "aws_sns_topic_subscription" "email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.notify_email
}

# ---- Budget (requires the run-id cost allocation tag to be activated, see runbook) --------

resource "aws_budgets_budget" "run" {
  name         = local.name
  budget_type  = "COST"
  limit_amount = format("%.2f", var.budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  cost_filter {
    name   = "TagKeyValue"
    values = [format("user:run-id$%s", var.run_id)]
  }

  dynamic "notification" {
    for_each = toset([50, 80, 100])
    content {
      comparison_operator       = "GREATER_THAN"
      threshold                 = notification.value
      threshold_type            = "PERCENTAGE"
      notification_type         = "ACTUAL"
      subscriber_sns_topic_arns = [aws_sns_topic.alerts.arn]
    }
  }
}

# ---- TTL teardown -------------------------------------------------------------------------

resource "aws_iam_role" "teardown" {
  name = "${local.name}-teardown"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "codebuild.amazonaws.com" }
      Action    = "sts:AssumeRole"
      Condition = { StringEquals = { "aws:SourceAccount" = local.account_id } }
    }]
  })
}

# Destroying VPC/EKS/IAM/KMS/S3 needs broad delete rights; the role is assumable only by
# CodeBuild in this account and exists only for the run (accepted risk, ADR-0010).
resource "aws_iam_role_policy_attachment" "teardown_power_user" {
  role       = aws_iam_role.teardown.name
  policy_arn = "arn:${local.partition}:iam::aws:policy/PowerUserAccess"
}

resource "aws_iam_role_policy_attachment" "teardown_iam" {
  role       = aws_iam_role.teardown.name
  policy_arn = "arn:${local.partition}:iam::aws:policy/IAMFullAccess"
}

resource "aws_codebuild_project" "teardown" {
  name          = "${local.name}-teardown"
  description   = "AIBox TTL teardown for run ${var.run_id}"
  service_role  = aws_iam_role.teardown.arn
  build_timeout = 90

  artifacts {
    type = "NO_ARTIFACTS"
  }

  environment {
    compute_type = "BUILD_GENERAL1_SMALL"
    image        = "aws/codebuild/amazonlinux-x86_64-standard:5.0"
    type         = "LINUX_CONTAINER"

    dynamic "environment_variable" {
      for_each = {
        AIBOX_TFVARS_B64     = base64encode(var.main_tfvars_json)
        STATE_BUCKET         = var.state_bucket
        MAIN_STATE_KEY       = var.main_state_key
        GUARDRAILS_STATE_KEY = var.guardrails_state_key
        AWS_REGION_NAME      = var.region
        CLUSTER_NAME         = var.cluster_name
        REPO_URL             = var.repo_url
        REVISION             = var.revision
        TOFU_VERSION         = var.tofu_version
        KUBECTL_VERSION      = var.kubectl_version
        ALERT_TOPIC_ARN      = aws_sns_topic.alerts.arn
        TF_VAR_profile       = var.profile
        TF_VAR_run_id        = var.run_id
        TF_VAR_owner         = var.owner
        TF_VAR_region        = var.region
        TF_VAR_cluster_name  = var.cluster_name
        TF_VAR_budget_usd    = tostring(var.budget_usd)
        TF_VAR_notify_email  = var.notify_email
        TF_VAR_ttl_expires   = var.ttl_expires_at
      }
      content {
        name  = environment_variable.key
        value = environment_variable.value
      }
    }
  }

  source {
    type      = "NO_SOURCE"
    buildspec = file("${path.module}/buildspec.yml")
  }
}

resource "aws_iam_role" "scheduler" {
  name = "${local.name}-scheduler"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "scheduler.amazonaws.com" }
      Action    = "sts:AssumeRole"
      Condition = { StringEquals = { "aws:SourceAccount" = local.account_id } }
    }]
  })
}

resource "aws_iam_role_policy" "scheduler" {
  name = "start-teardown"
  role = aws_iam_role.scheduler.id
  policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "codebuild:StartBuild", Resource = aws_codebuild_project.teardown.arn }]
  })
}

resource "aws_scheduler_schedule" "teardown" {
  name                         = "${local.name}-ttl"
  description                  = "AIBox TTL teardown for run ${var.run_id}"
  schedule_expression          = "at(${formatdate("YYYY-MM-DD'T'hh:mm:ss", var.ttl_expires_at)})"
  schedule_expression_timezone = "UTC"
  action_after_completion      = "DELETE"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = "arn:${local.partition}:scheduler:::aws-sdk:codebuild:startBuild"
    role_arn = aws_iam_role.scheduler.arn
    input    = jsonencode({ ProjectName = aws_codebuild_project.teardown.name })

    retry_policy {
      maximum_retry_attempts       = 3
      maximum_event_age_in_seconds = 3600
    }
  }
}
