terraform {
  required_version = ">= 1.10.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.72.0"
    }
  }
}

provider "aws" {
  region = var.region
  default_tags {
    tags = {
      project = "aibox"
      owner   = var.owner
      profile = "bootstrap"
      run-id  = "bootstrap"
    }
  }
}
