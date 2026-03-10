terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}

# Get current AWS account information
data "aws_caller_identity" "current" {}

# Get available AWS regions
data "aws_regions" "available" {
  all_regions = false
}

# Get available availability zones in the current region
data "aws_availability_zones" "available" {
  state = "available"
}

# Output the information
output "account_id" {
  description = "AWS Account ID"
  value       = data.aws_caller_identity.current.account_id
}

output "caller_arn" {
  description = "ARN of the caller (assumed role)"
  value       = data.aws_caller_identity.current.arn
}

output "caller_user_id" {
  description = "Unique ID of the caller"
  value       = data.aws_caller_identity.current.user_id
}

output "available_regions" {
  description = "List of available AWS regions"
  value       = data.aws_regions.available.names
}

output "availability_zones" {
  description = "List of availability zones in the current region"
  value       = data.aws_availability_zones.available.names
}
