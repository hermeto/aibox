terraform {
  required_version = ">= 1.10.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.72.0"
    }
  }

  # Partial configuration; aibox up passes bucket, key (<profile>/<cluster>-guardrails.tfstate)
  # and region. Applied before and destroyed after the main stack.
  backend "s3" {
    use_lockfile = true
    encrypt      = true
  }
}

provider "aws" {
  region = var.region
  default_tags {
    tags = {
      project = "aibox"
      run-id  = var.run_id
      owner   = var.owner
      profile = var.profile
    }
  }
}
