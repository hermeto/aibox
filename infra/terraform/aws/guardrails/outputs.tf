output "teardown_role_arn" {
  description = "Role the teardown job assumes (granted cluster access by the main stack)."
  value       = aws_iam_role.teardown.arn
}

output "teardown_schedule" {
  description = "EventBridge Scheduler expression of the TTL teardown."
  value       = aws_scheduler_schedule.teardown.schedule_expression
}

output "alerts_topic_arn" {
  description = "SNS topic for budget and teardown alerts."
  value       = aws_sns_topic.alerts.arn
}

output "budget_name" {
  description = "AWS Budget name."
  value       = aws_budgets_budget.run.name
}
