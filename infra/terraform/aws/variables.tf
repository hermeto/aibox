variable "profile" {
  description = "AIBox profile name (aws-dev, aws-bench)."
  type        = string
  validation {
    condition     = can(regex("^aws-[a-z0-9-]+$", var.profile))
    error_message = "profile must look like aws-<name>."
  }
}

variable "run_id" {
  description = "Unique ID of this bring-up, used in tags (set by aibox up)."
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9-]{8,40}$", var.run_id))
    error_message = "run_id must be 8-40 characters of [a-z0-9-]."
  }
}

variable "owner" {
  description = "Who owns the run (email or user name), used in tags."
  type        = string
  validation {
    condition     = length(var.owner) > 0 && length(var.owner) <= 128
    error_message = "owner must be 1-128 characters."
  }
}

variable "region" {
  description = "AWS region."
  type        = string
  validation {
    condition     = can(regex("^[a-z]{2}(-gov)?-[a-z]+-[0-9]$", var.region))
    error_message = "region must be an AWS region name such as us-east-1."
  }
}

variable "cluster_name" {
  description = "EKS cluster name."
  type        = string
  validation {
    condition     = can(regex("^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$", var.cluster_name))
    error_message = "cluster_name must be a DNS label of at most 40 characters."
  }
}

variable "kubernetes_version" {
  description = "EKS Kubernetes version (MAJOR.MINOR)."
  type        = string
  validation {
    condition     = can(regex("^1\\.[0-9]{2}$", var.kubernetes_version))
    error_message = "kubernetes_version must look like 1.35."
  }
}

variable "admin_cidrs" {
  description = "CIDRs allowed to reach the EKS public endpoint (aibox up uses the operator's egress IP /32)."
  type        = list(string)
  validation {
    condition     = length(var.admin_cidrs) > 0 && alltrue([for c in var.admin_cidrs : can(cidrhost(c, 0)) && c != "0.0.0.0/0"])
    error_message = "admin_cidrs must contain at least one valid CIDR and must not contain 0.0.0.0/0."
  }
}

variable "az_count" {
  description = "Number of Availability Zones."
  type        = number
  default     = 3
  validation {
    condition     = var.az_count >= 2 && var.az_count <= 3
    error_message = "az_count must be 2 or 3."
  }
}

variable "vpc_cidr" {
  description = "VPC CIDR block (a /16 is split into /20 private and /24 public subnets)."
  type        = string
  default     = "10.42.0.0/16"
  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0)) && endswith(var.vpc_cidr, "/16")
    error_message = "vpc_cidr must be a valid /16."
  }
}

variable "system_node_instance_types" {
  description = "Instance types of the on-demand CPU system node group."
  type        = list(string)
  default     = ["m7i.xlarge"]
}

variable "system_node_min" {
  description = "Minimum system nodes."
  type        = number
  default     = 2
}

variable "system_node_desired" {
  description = "Desired system nodes."
  type        = number
  default     = 2
}

variable "system_node_max" {
  description = "Maximum system nodes."
  type        = number
  default     = 3
  validation {
    condition     = var.system_node_min >= 1 && var.system_node_min <= var.system_node_desired && var.system_node_desired <= var.system_node_max && var.system_node_max <= 10
    error_message = "require 1 <= system_node_min <= system_node_desired <= system_node_max <= 10."
  }
}
