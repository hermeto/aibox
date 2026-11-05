output "cluster_name" {
  description = "EKS cluster name."
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "EKS API server endpoint."
  value       = module.eks.cluster_endpoint
}

output "cluster_certificate_authority_data" {
  description = "Base64 CA bundle of the cluster."
  value       = module.eks.cluster_certificate_authority_data
}

output "oidc_issuer_url" {
  description = "OIDC issuer URL of the cluster."
  value       = module.eks.cluster_oidc_issuer_url
}

output "region" {
  description = "AWS region."
  value       = var.region
}

output "vpc_id" {
  description = "VPC ID."
  value       = module.network.vpc_id
}

output "private_subnet_ids" {
  description = "Private (node) subnet IDs."
  value       = module.network.private_subnets
}

output "weights_bucket" {
  description = "S3 bucket for model weights."
  value       = module.storage.bucket_name
}

output "weights_kms_key_arn" {
  description = "KMS key encrypting the weights bucket."
  value       = module.storage.kms_key_arn
}

output "karpenter_queue_name" {
  description = "SQS queue for Karpenter interruption handling."
  value       = module.karpenter.queue_name
}

output "karpenter_node_role_name" {
  description = "IAM role name for Karpenter-launched nodes."
  value       = module.karpenter.node_iam_role_name
}

output "role_arns" {
  description = "Pod Identity role ARNs per platform component."
  value       = module.iam.role_arns
}

output "tags" {
  description = "Tags applied to every resource."
  value       = local.tags
}
