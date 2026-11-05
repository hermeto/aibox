# Input validation (no AWS access: providers are mocked, upstream modules overridden).
mock_provider "aws" {
  mock_data "aws_availability_zones" {
    defaults = { names = ["us-east-1a", "us-east-1b", "us-east-1c"] }
  }
  mock_data "aws_caller_identity" {
    defaults = { account_id = "000000000000", arn = "arn:aws:iam::000000000000:user/test", user_id = "AIDAEXAMPLE" }
  }
  mock_data "aws_iam_session_context" {
    defaults = { issuer_arn = "arn:aws:iam::000000000000:user/test" }
  }
  mock_data "aws_partition" {
    defaults = { partition = "aws" }
  }
  mock_data "aws_region" {
    defaults = { region = "us-east-1" }
  }
  mock_resource "aws_iam_role" {
    defaults = { arn = "arn:aws:iam::000000000000:role/mock" }
  }
  mock_resource "aws_iam_policy" {
    defaults = { arn = "arn:aws:iam::000000000000:policy/mock" }
  }
  mock_resource "aws_sqs_queue" {
    defaults = { arn = "arn:aws:sqs:us-east-1:000000000000:mock", url = "https://sqs.us-east-1.amazonaws.com/000000000000/mock" }
  }
  mock_resource "aws_kms_key" {
    defaults = { arn = "arn:aws:kms:us-east-1:000000000000:key/00000000-0000-0000-0000-000000000000", key_id = "00000000-0000-0000-0000-000000000000" }
  }
  mock_resource "aws_s3_bucket" {
    defaults = { arn = "arn:aws:s3:::aibox-weights-aibox-test-000000000000" }
  }
  mock_resource "aws_eks_cluster" {
    defaults = {
      arn                   = "arn:aws:eks:us-east-1:000000000000:cluster/aibox-test"
      endpoint              = "https://example.invalid"
      certificate_authority = [{ data = "Y2E=" }]
      identity              = [{ oidc = [{ issuer = "https://oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE" }] }]
    }
  }
  mock_resource "aws_iam_openid_connect_provider" {
    defaults = { arn = "arn:aws:iam::000000000000:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE" }
  }
  mock_resource "aws_cloudwatch_event_rule" {
    defaults = { arn = "arn:aws:events:us-east-1:000000000000:rule/mock" }
  }
  mock_resource "aws_iam_instance_profile" {
    defaults = { arn = "arn:aws:iam::000000000000:instance-profile/mock" }
  }
  mock_resource "aws_launch_template" {
    defaults = { id = "lt-00000000000000000", arn = "arn:aws:ec2:us-east-1:000000000000:launch-template/lt-00000000000000000", latest_version = 1 }
  }
  mock_resource "aws_cloudwatch_log_group" {
    defaults = { arn = "arn:aws:logs:us-east-1:000000000000:log-group:mock" }
  }
  mock_data "aws_eks_addon_version" {
    defaults = { version = "v1.0.0-eksbuild.1" }
  }
  mock_data "aws_iam_policy_document" {
    defaults = { json = "{\"Version\":\"2012-10-17\",\"Statement\":[]}" }
  }
}
mock_provider "tls" {}
mock_provider "time" {}
mock_provider "null" {}


variables {
  profile            = "aws-dev"
  run_id             = "aws-dev-202611050000-abcd"
  owner              = "maintainer@example.com"
  region             = "us-east-1"
  cluster_name       = "aibox-test"
  kubernetes_version = "1.35"
  admin_cidrs        = ["203.0.113.10/32"]
}

run "valid_inputs_plan" {
  command = plan
}

run "rejects_open_endpoint" {
  command = plan
  variables {
    admin_cidrs = ["0.0.0.0/0"]
  }
  expect_failures = [var.admin_cidrs]
}

run "rejects_empty_admin_cidrs" {
  command = plan
  variables {
    admin_cidrs = []
  }
  expect_failures = [var.admin_cidrs]
}

run "rejects_bad_profile" {
  command = plan
  variables {
    profile = "local"
  }
  expect_failures = [var.profile]
}

run "rejects_bad_run_id" {
  command = plan
  variables {
    run_id = "Bad_ID"
  }
  expect_failures = [var.run_id]
}

run "rejects_bad_kubernetes_version" {
  command = plan
  variables {
    kubernetes_version = "latest"
  }
  expect_failures = [var.kubernetes_version]
}

run "rejects_bad_node_sizes" {
  command = plan
  variables {
    system_node_min     = 3
    system_node_desired = 2
  }
  expect_failures = [var.system_node_max]
}

run "rejects_single_az" {
  command = plan
  variables {
    az_count = 1
  }
  expect_failures = [var.az_count]
}
