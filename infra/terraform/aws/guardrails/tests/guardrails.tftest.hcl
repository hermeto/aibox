mock_provider "aws" {
  mock_data "aws_caller_identity" {
    defaults = { account_id = "000000000000" }
  }
  mock_data "aws_partition" {
    defaults = { partition = "aws" }
  }
  mock_resource "aws_kms_key" {
    defaults = { arn = "arn:aws:kms:us-east-1:000000000000:key/00000000-0000-0000-0000-000000000000" }
  }
  mock_resource "aws_sns_topic" {
    defaults = { arn = "arn:aws:sns:us-east-1:000000000000:aibox-alerts" }
  }
  mock_resource "aws_iam_role" {
    defaults = { arn = "arn:aws:iam::000000000000:role/aibox-teardown" }
  }
  mock_resource "aws_codebuild_project" {
    defaults = { arn = "arn:aws:codebuild:us-east-1:000000000000:project/aibox-teardown" }
  }
}

variables {
  profile              = "aws-dev"
  run_id               = "aws-dev-202611050100-abcd"
  owner                = "maintainer@example.com"
  region               = "us-east-1"
  cluster_name         = "aibox-dev"
  budget_usd           = 50
  notify_email         = "maintainer@example.com"
  ttl_expires_at       = "2026-11-05T05:00:00Z"
  main_tfvars_json     = "{\"run_id\":\"aws-dev-202611050100-abcd\"}"
  state_bucket         = "aibox-tfstate-000000000000-us-east-1"
  main_state_key       = "aws-dev/aibox-dev.tfstate"
  guardrails_state_key = "aws-dev/aibox-dev-guardrails.tfstate"
  repo_url             = "https://github.com/hermeto/aibox.git"
  revision             = "0123456789abcdef0123456789abcdef01234567"
}

run "budget_and_teardown" {
  command = plan

  assert {
    condition     = aws_budgets_budget.run.limit_amount == "50.00" && contains(one(aws_budgets_budget.run.cost_filter).values, "user:run-id$aws-dev-202611050100-abcd")
    error_message = "budget must be the run amount filtered by the run-id tag"
  }
  assert {
    condition     = toset([for n in aws_budgets_budget.run.notification : n.threshold]) == toset([50, 80, 100])
    error_message = "alerts at 50/80/100%"
  }
  assert {
    condition     = aws_scheduler_schedule.teardown.schedule_expression == "at(2026-11-05T05:00:00)" && aws_scheduler_schedule.teardown.action_after_completion == "DELETE"
    error_message = "one-time teardown at the TTL expiry, deleted after completion"
  }
  assert {
    condition     = aws_codebuild_project.teardown.build_timeout == 90 && one(aws_codebuild_project.teardown.source).type == "NO_SOURCE"
    error_message = "teardown project"
  }
  assert {
    condition     = aws_sns_topic_subscription.email.endpoint == "maintainer@example.com"
    error_message = "alerts go to the operator"
  }
}

run "rejects_zero_budget" {
  command = plan
  variables {
    budget_usd = 0
  }
  expect_failures = [var.budget_usd]
}

run "rejects_bad_email" {
  command = plan
  variables {
    notify_email = "nobody"
  }
  expect_failures = [var.notify_email]
}
