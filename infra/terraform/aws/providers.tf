provider "aws" {
  region = var.region

  # Every taggable resource carries these tags; feature 005 budgets and orphan detection
  # key on them.
  default_tags {
    tags = local.tags
  }
}

locals {
  tags = {
    project = "aibox"
    run-id  = var.run_id
    owner   = var.owner
    profile = var.profile
  }
}
