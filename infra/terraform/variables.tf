variable "aws_region" {
  type    = string
  default = "ap-southeast-3"
  validation {
    condition     = var.aws_region == "ap-southeast-3"
    error_message = "Demo ini hanya memakai AWS Region Jakarta (ap-southeast-3)."
  }
}

variable "project_name" {
  type    = string
  default = "firman-message-demo"
}

variable "kubernetes_version" {
  type    = string
  default = "1.34"
}

variable "cluster_public_access_cidrs" {
  description = "Your admin IP/CIDR for kubectl. Example: [\"203.0.113.10/32\"]."
  type        = list(string)
  validation {
    condition     = length(var.cluster_public_access_cidrs) > 0 && alltrue([for cidr in var.cluster_public_access_cidrs : can(cidrhost(cidr, 0)) && cidr != "0.0.0.0/0"])
    error_message = "Provide at least one valid CIDR; unrestricted access is not allowed."
  }
}
