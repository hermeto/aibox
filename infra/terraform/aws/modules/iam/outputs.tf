output "role_arns" {
  description = "Role ARN per component."
  value       = { for k, r in aws_iam_role.component : k => r.arn }
}

output "service_accounts" {
  description = "Namespace/service account per component."
  value       = { for k, c in local.components : k => "${c.namespace}/${c.service_account}" }
}
