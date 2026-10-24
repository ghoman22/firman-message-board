variable "aws_region" {
  type    = string
  default = "ap-southeast-3"
  validation {
    condition     = var.aws_region == "ap-southeast-3"
    error_message = "Demo ini hanya memakai Region Jakarta."
  }
}

variable "project_name" {
  type    = string
  default = "firman-message-demo"
}

variable "github_repository" {
  description = "GitHub owner/repository, used to restrict the CI OIDC role."
  type        = string
  validation {
    condition     = can(regex("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", var.github_repository))
    error_message = "Use owner/repository format."
  }
}

variable "existing_github_oidc_provider_arn" {
  description = "Optional ARN of an existing GitHub Actions OIDC provider in this account."
  type        = string
  default     = null
  validation {
    condition     = var.existing_github_oidc_provider_arn == null || can(regex("^arn:aws:iam::[0-9]{12}:oidc-provider/token[.]actions[.]githubusercontent[.]com$", var.existing_github_oidc_provider_arn))
    error_message = "Use the full GitHub Actions OIDC provider ARN."
  }
}
