output "state_bucket" {
  description = "Name of the Terraform state bucket (aibox up derives the same name)."
  value       = aws_s3_bucket.state.bucket
}
