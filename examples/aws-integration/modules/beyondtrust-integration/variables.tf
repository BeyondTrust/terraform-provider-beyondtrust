# Module Variables

variable "external_id" {
  description = "External ID for AWS integration"
  type        = string
  sensitive   = true
}

variable "integration_name" {
  description = "Name for the BeyondTrust AWS integration"
  type        = string
}

variable "smop_integration_role_arn" {
  description = "ARN of the AWS IAM role for SMOP integration"
  type        = string
}

variable "developer_role_arn" {
  description = "ARN of the developer role to assume"
  type        = string
}

variable "admin_role_arn" {
  description = "ARN of the admin role to assume"
  type        = string
}

variable "developer_policy_arns" {
  description = "Policy ARNs for developer access"
  type        = list(string)
}

variable "environment" {
  description = "Environment name"
  type        = string
}
