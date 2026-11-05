variable "cluster_name" {
  description = "Cluster name, used to derive the bucket name."
  type        = string
}

variable "force_destroy" {
  description = "Allow destroying a non-empty bucket (aibox down empties nothing by itself)."
  type        = bool
  default     = true
}
