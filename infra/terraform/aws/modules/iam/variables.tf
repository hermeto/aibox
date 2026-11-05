variable "cluster_name" {
  description = "EKS cluster name for the Pod Identity associations."
  type        = string
}

variable "weights_bucket_arn" {
  description = "ARN of the model weights bucket."
  type        = string
}

variable "weights_kms_arn" {
  description = "ARN of the KMS key encrypting the weights bucket."
  type        = string
}
