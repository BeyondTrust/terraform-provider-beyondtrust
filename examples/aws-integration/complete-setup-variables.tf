# Variables for Complete AWS Integration Setup

variable "aws_region" {
  description = "AWS region for resources"
  type        = string
  default     = "us-east-1"
}

variable "smop_bridge_account_id" {
  description = "AWS account ID of the SMOP bridge account"
  type        = string
  default     = "109876543210" # Sandbox/dev bridge account
  # Use "012345678901" for production

  validation {
    condition     = can(regex("^[0-9]{12}$", var.smop_bridge_account_id))
    error_message = "Bridge account ID must be a 12-digit AWS account ID"
  }
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"

  validation {
    condition     = contains(["production", "staging", "development"], var.environment)
    error_message = "Environment must be one of: production, staging, development"
  }
}

variable "tags" {
  description = "Tags to apply to AWS resources"
  type        = map(string)
  default = {
    ManagedBy   = "Terraform"
    Environment = "Production"
    Purpose     = "SMOP-Integration"
  }
}

variable "skip_beyondtrust" {
  description = "Skip BeyondTrust resources (use for terraform init)"
  type        = bool
  default     = false
}
