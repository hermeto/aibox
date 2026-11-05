variable "region" {
  description = "AWS region of the state bucket (use the region of your runs)."
  type        = string
  validation {
    condition     = can(regex("^[a-z]{2}(-gov)?-[a-z]+-[0-9]$", var.region))
    error_message = "region must be an AWS region name such as us-east-1."
  }
}

variable "owner" {
  description = "Owner tag value (email or user name)."
  type        = string
  validation {
    condition     = length(var.owner) > 0
    error_message = "owner must not be empty."
  }
}
