variable "aws_region" {
  type    = string
  default = "ap-southeast-3"
}

variable "admin_cidr" {
  description = "Public IP of the operator, e.g. 203.0.113.10/32. Only SSH is exposed publicly."
  type        = string
  validation {
    condition     = can(cidrhost(var.admin_cidr, 0)) && var.admin_cidr != "0.0.0.0/0"
    error_message = "Use a valid, restricted CIDR."
  }
}

variable "ssh_public_key_path" {
  description = "Path to an existing SSH public key on the operator computer."
  type        = string
}
