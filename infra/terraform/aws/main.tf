data "aws_availability_zones" "available" {
  state = "available"
  filter {
    name   = "opt-in-status"
    values = ["opt-in-not-required"]
  }
}

locals {
  azs             = slice(data.aws_availability_zones.available.names, 0, var.az_count)
  private_subnets = [for i in range(var.az_count) : cidrsubnet(var.vpc_cidr, 4, i)]
  public_subnets  = [for i in range(var.az_count) : cidrsubnet(var.vpc_cidr, 8, 48 + i)]
}

module "network" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "6.7.3"

  name = var.cluster_name
  cidr = var.vpc_cidr
  azs  = local.azs

  private_subnets = local.private_subnets
  public_subnets  = local.public_subnets

  enable_nat_gateway   = true
  single_nat_gateway   = true # production HA is out of scope (product spec §A.2)
  enable_dns_hostnames = true

  public_subnet_tags = {
    "kubernetes.io/role/elb" = "1"
  }
  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = "1"
    "karpenter.sh/discovery"          = var.cluster_name
  }
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "21.29.0"

  name               = var.cluster_name
  kubernetes_version = var.kubernetes_version

  vpc_id                   = module.network.vpc_id
  subnet_ids               = module.network.private_subnets
  control_plane_subnet_ids = module.network.private_subnets

  endpoint_public_access       = true
  endpoint_public_access_cidrs = var.admin_cidrs
  endpoint_private_access      = true

  enable_cluster_creator_admin_permissions = true
  # Workload identities use EKS Pod Identity (modules/iam), not IRSA.
  enable_irsa       = false
  enabled_log_types = ["api", "audit", "authenticator"]

  addons = {
    coredns                = {}
    kube-proxy             = {}
    eks-pod-identity-agent = { before_compute = true }
    vpc-cni                = { before_compute = true }
  }

  eks_managed_node_groups = {
    system = {
      instance_types = var.system_node_instance_types
      capacity_type  = "ON_DEMAND"
      min_size       = var.system_node_min
      max_size       = var.system_node_max
      desired_size   = var.system_node_desired
      labels = {
        "aibox.dev/role" = "system"
      }
      metadata_options = {
        http_endpoint               = "enabled"
        http_tokens                 = "required"
        http_put_response_hop_limit = 1
      }
    }
  }

  node_security_group_tags = {
    "karpenter.sh/discovery" = var.cluster_name
  }
}

module "karpenter" {
  source  = "terraform-aws-modules/eks/aws//modules/karpenter"
  version = "21.29.0"

  cluster_name                    = module.eks.cluster_name
  create_pod_identity_association = true
  namespace                       = "karpenter"
  service_account                 = "karpenter"

  node_iam_role_use_name_prefix = false
  node_iam_role_name            = "${var.cluster_name}-karpenter-node"
  node_iam_role_additional_policies = {
    AmazonSSMManagedInstanceCore = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
  }
}

module "storage" {
  source = "./modules/storage"

  cluster_name = var.cluster_name
}

module "iam" {
  source = "./modules/iam"

  cluster_name       = module.eks.cluster_name
  weights_bucket_arn = module.storage.bucket_arn
  weights_kms_arn    = module.storage.kms_key_arn
}
