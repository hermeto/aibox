# Pod Identity roles: trust, component set and least-privilege scoping.
mock_provider "aws" {
  mock_data "aws_caller_identity" {
    defaults = { account_id = "000000000000" }
  }
  mock_data "aws_partition" {
    defaults = { partition = "aws" }
  }
  mock_data "aws_region" {
    defaults = { region = "us-east-1" }
  }
  mock_data "aws_iam_policy_document" {
    defaults = { json = "{\"Version\":\"2012-10-17\",\"Statement\":[]}" }
  }
  mock_resource "aws_iam_role" {
    defaults = { arn = "arn:aws:iam::000000000000:role/mock" }
  }
}

variables {
  cluster_name       = "aibox-test"
  weights_bucket_arn = "arn:aws:s3:::aibox-weights-aibox-test-000000000000"
  weights_kms_arn    = "arn:aws:kms:us-east-1:000000000000:key/00000000-0000-0000-0000-000000000000"
}

run "roles_are_scoped" {
  command = apply

  module {
    source = "./modules/iam"
  }

  assert {
    condition     = toset(keys(aws_iam_role.component)) == toset(["external-secrets", "otel-collector", "model-sync", "model-server"])
    error_message = "exactly four component roles (no KEDA role)"
  }
  assert {
    condition     = one(data.aws_iam_policy_document.trust.statement[0].principals).identifiers == toset(["pods.eks.amazonaws.com"])
    error_message = "roles must trust only the EKS Pod Identity service"
  }
  assert {
    condition     = aws_eks_pod_identity_association.component["model-server"].namespace == "aibox-serving" && aws_eks_pod_identity_association.component["model-server"].service_account == "model-server"
    error_message = "model-server association"
  }
  assert {
    condition     = alltrue([for s in data.aws_iam_policy_document.component["model-server"].statement : alltrue([for a in s.actions : contains(["s3:ListBucket", "s3:GetObject", "kms:Decrypt"], a)])])
    error_message = "model servers must only read weights"
  }
  assert {
    condition     = alltrue([for s in data.aws_iam_policy_document.component["model-sync"].statement : alltrue([for r in s.resources : startswith(r, "arn:aws:s3:::aibox-weights-aibox-test-000000000000") || startswith(r, "arn:aws:kms:")])])
    error_message = "model sync must be limited to the weights bucket and its key"
  }
  assert {
    condition     = one(data.aws_iam_policy_document.component["external-secrets"].statement).resources == toset(["arn:aws:secretsmanager:us-east-1:000000000000:secret:aibox/aibox-test/*"])
    error_message = "External Secrets must only read secrets under aibox/<cluster>/"
  }
}
