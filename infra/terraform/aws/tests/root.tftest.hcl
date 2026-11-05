# Root wiring: outputs, tags and subnet layout with mocked providers (apply is mocked).
mock_provider "aws" {
  mock_data "aws_availability_zones" {
    defaults = { names = ["us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d"] }
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
  profile            = "aws-bench"
  run_id             = "aws-bench-202611050000-beef"
  owner              = "maintainer@example.com"
  region             = "us-east-1"
  cluster_name       = "aibox-test"
  kubernetes_version = "1.35"
  admin_cidrs        = ["203.0.113.10/32"]
}

run "tags_and_outputs" {
  command = apply

  assert {
    condition     = output.tags == { project = "aibox", run-id = "aws-bench-202611050000-beef", owner = "maintainer@example.com", profile = "aws-bench" }
    error_message = "default tags must be project, run-id, owner and profile"
  }
  assert {
    condition     = output.weights_bucket == "aibox-weights-aibox-test-000000000000"
    error_message = "weights bucket name must include the cluster name and account"
  }
  assert {
    condition     = toset(keys(output.role_arns)) == toset(["external-secrets", "otel-collector", "model-sync", "model-server"])
    error_message = "Pod Identity roles must exist for exactly the four AWS-dependent components"
  }
  assert {
    condition     = output.cluster_name == "aibox-test" && output.karpenter_queue_name == "Karpenter-aibox-test"
    error_message = "cluster and Karpenter outputs must be passed through"
  }
}

run "subnet_layout_uses_three_azs" {
  command = plan

  assert {
    condition     = length(local.azs) == 3 && local.azs[0] == "us-east-1a"
    error_message = "the first three available AZs must be used"
  }
  assert {
    condition     = local.private_subnets == ["10.42.0.0/20", "10.42.16.0/20", "10.42.32.0/20"]
    error_message = "private subnets must be /20s at the start of the VPC"
  }
  assert {
    condition     = local.public_subnets == ["10.42.48.0/24", "10.42.49.0/24", "10.42.50.0/24"]
    error_message = "public subnets must be /24s after the private range"
  }
}
