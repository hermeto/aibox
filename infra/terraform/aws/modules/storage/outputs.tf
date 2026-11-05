output "bucket_name" {
  description = "Weights bucket name."
  value       = aws_s3_bucket.weights.bucket
}

output "bucket_arn" {
  description = "Weights bucket ARN."
  value       = aws_s3_bucket.weights.arn
}

output "kms_key_arn" {
  description = "KMS key ARN for the weights bucket."
  value       = aws_kms_key.weights.arn
}
