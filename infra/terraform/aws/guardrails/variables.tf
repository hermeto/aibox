variable "profile" {
  description = "AIBox profile name."
  type        = string
}

variable "run_id" {
  description = "Run ID (also the budget's tag filter)."
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9-]{8,40}$", var.run_id))
    error_message = "run_id must be 8-40 characters of [a-z0-9-]."
  }
}

variable "owner" {
  description = "Owner tag value."
  type        = string
}

variable "region" {
  description = "AWS region."
  type        = string
}

variable "cluster_name" {
  description = "EKS cluster name of the main stack."
  type        = string
}

variable "budget_usd" {
  description = "Budget for this run in USD (alerts at 50/80/100%)."
  type        = number
  validation {
    condition     = var.budget_usd > 0 && var.budget_usd <= 10000
    error_message = "budget_usd must be in (0, 10000]."
  }
}

variable "notify_email" {
  description = "E-mail receiving budget and teardown alerts."
  type        = string
  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.notify_email))
    error_message = "notify_email must be an e-mail address."
  }
}

variable "ttl_expires_at" {
  description = "RFC3339 UTC time when the teardown runs (start + TTL, at most 24h after the start)."
  type        = string
  validation {
    condition     = can(formatdate("YYYY", var.ttl_expires_at))
    error_message = "ttl_expires_at must be an RFC3339 timestamp."
  }
}

variable "main_tfvars_json" {
  description = "JSON inputs of the main stack (the teardown destroys it with these)."
  type        = string
  sensitive   = false
}

variable "state_bucket" {
  description = "S3 state bucket."
  type        = string
}

variable "main_state_key" {
  description = "State key of the main stack."
  type        = string
}

variable "guardrails_state_key" {
  description = "State key of this stack."
  type        = string
}

variable "repo_url" {
  description = "Public git repository cloned by the teardown job."
  type        = string
  validation {
    condition     = startswith(var.repo_url, "https://")
    error_message = "repo_url must be an https URL."
  }
}

variable "revision" {
  description = "Git revision (the run's commit) checked out by the teardown job."
  type        = string
}

variable "tofu_version" {
  description = "OpenTofu version installed by the teardown job (checksum-verified)."
  type        = string
  default     = "1.13.8"
}

variable "kubectl_version" {
  description = "kubectl version used to drain the cluster before destroy."
  type        = string
  default     = "v1.37.2"
}
