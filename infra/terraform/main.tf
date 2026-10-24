data "aws_availability_zones" "available" { state = "available" }

locals {
  azs = slice(data.aws_availability_zones.available.names, 0, 2)
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "6.3.0"

  name                    = "${var.project_name}-vpc"
  cidr                    = "10.42.0.0/16"
  azs                     = local.azs
  private_subnets         = ["10.42.1.0/24", "10.42.2.0/24"]
  public_subnets          = ["10.42.101.0/24", "10.42.102.0/24"]
  enable_nat_gateway      = false
  map_public_ip_on_launch = true
  enable_dns_hostnames    = true
  public_subnet_tags      = { "kubernetes.io/role/elb" = "1" }
  private_subnet_tags     = { "kubernetes.io/role/internal-elb" = "1" }
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "21.26.0"

  name                                     = var.project_name
  kubernetes_version                       = var.kubernetes_version
  endpoint_public_access                   = true
  endpoint_public_access_cidrs             = var.cluster_public_access_cidrs
  endpoint_private_access                  = true
  enable_cluster_creator_admin_permissions = true
  vpc_id                                   = module.vpc.vpc_id
  subnet_ids                               = module.vpc.public_subnets
  control_plane_subnet_ids                 = module.vpc.private_subnets
  addons = {
    coredns    = {}
    kube-proxy = {}
    vpc-cni    = { before_compute = true }
  }
  eks_managed_node_groups = {
    demo = {
      instance_types = ["t3.small"]
      ami_type       = "AL2023_x86_64_STANDARD"
      min_size       = 2
      max_size       = 3
      desired_size   = 2
    }
  }
}

